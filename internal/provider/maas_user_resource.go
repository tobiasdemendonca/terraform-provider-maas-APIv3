package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

var _ resource.Resource = (*maasUserResource)(nil)
var _ resource.ResourceWithImportState = (*maasUserResource)(nil)

func NewMaasUserResource() resource.Resource {
	return &maasUserResource{}
}

type maasUserResource struct {
	client *maasclientv3.ClientWithResponses
}

type maasUserResourceModel struct {
	Email                 types.String `tfsdk:"email"`
	FirstName             types.String `tfsdk:"first_name"`
	Groups                types.List   `tfsdk:"groups"`
	Id                    types.Int64  `tfsdk:"id"`
	LastName              types.String `tfsdk:"last_name"`
	PasswordWo            types.String `tfsdk:"password_wo"`
	PasswordWoVersion     types.String `tfsdk:"password_wo_version"`
	TransferResourcesToWo types.Int64  `tfsdk:"transfer_resources_to_wo"`
	Username              types.String `tfsdk:"username"`
}

func (r *maasUserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *maasUserResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a MAAS user.",
		Attributes: map[string]schema.Attribute{
			"email": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The user email address. Can be null in MAAS.",
			},
			"first_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The user first name.",
			},
			"groups": schema.ListAttribute{
				ElementType:         types.Int64Type,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The IDs of the groups the user is a member of. Full management: any group not listed here is removed. Absent means the user is in no groups. Administrator status is granted by including the administrators group id.",
				Default:             listdefault.StaticValue(types.ListValueMust(types.Int64Type, []attr.Value{})),
			},
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The user ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"last_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The user last name.",
			},
			"password_wo": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				WriteOnly: true,
				MarkdownDescription: "The user password. Required on creation; not stored in Terraform state. " +
					"On update, the password is only sent to the API when `password_wo_version` changes, " +
					"otherwise it is left unchanged.",
			},
			"password_wo_version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Bump this value to force `password_wo` to be applied on the next update. Preserved in state across reads.",
			},
			"transfer_resources_to_wo": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "The id of the user to transfer resources to when this user is deleted. Not stored in MAAS; preserved in Terraform state and used only at delete time.",
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique username.",
			},
		},
	}
}

func (r *maasUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	username := req.ID
	users, err := r.client.ListUsersWithResponse(ctx, &maasclientv3.ListUsersParams{
		UsernameOrEmail: &username,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error importing user", err.Error())
		return
	}
	if users.JSON200 == nil {
		resp.Diagnostics.AddError("Error importing user", apiError(users.Status(), users.Body))
		return
	}
	for _, u := range users.JSON200.Items {
		if u.Username == username {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.Int64Value(int64(u.Id)))...)
			return
		}
	}
	resp.Diagnostics.AddError("Error importing user",
		fmt.Sprintf("No user with username %q was found.", username))
}

func (r *maasUserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(MaasProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected MaasProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	r.client = data.Client
}

func (r *maasUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data maasUserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// WriteOnly attributes are nullified in the plan by the framework; read
	// them from config.
	var config maasUserResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.PasswordWo = config.PasswordWo

	body := maasclientv3.CreateUserJSONRequestBody{
		Username:  data.Username.ValueString(),
		FirstName: data.FirstName.ValueString(),
		LastName:  data.LastName.ValueString(),
		Email:     optionalString(data.Email),
		Password:  data.PasswordWo.ValueString(),
		Groups:    optionalInt64List(data.Groups),
	}

	apiResp, err := r.client.CreateUserWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating user", err.Error())
		return
	}
	if apiResp.StatusCode() == 409 {
		resp.Diagnostics.AddError("Error creating user",
			fmt.Sprintf("A user with the username %q already exists. Use a different username or import the existing user with: terraform import maas_user.<name> %s", data.Username.ValueString(), data.Username.ValueString()))
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating user", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenUser(apiResp.JSON201, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *maasUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data maasUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetUserWithResponse(ctx, int(data.Id.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading user", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading user", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	// password_wo is WriteOnly: not in state, not recoverable. password_wo_version
	// and transfer_resources_to_wo are not returned by the API, so preserve the
	// prior state values.
	prevPasswordVersion := data.PasswordWoVersion
	prevTransfer := data.TransferResourcesToWo

	flattenUser(apiResp.JSON200, &data)

	data.PasswordWoVersion = prevPasswordVersion
	data.TransferResourcesToWo = prevTransfer

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *maasUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data maasUserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state maasUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// WriteOnly attributes are nullified in the plan by the framework; read
	// them from config.
	var config maasUserResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.PasswordWo = config.PasswordWo
	body := maasclientv3.UpdateUserJSONRequestBody{
		Username:  data.Username.ValueString(),
		FirstName: data.FirstName.ValueString(),
		LastName:  data.LastName.ValueString(),
		Email:     optionalString(data.Email),
		Groups:    optionalInt64List(data.Groups),
	}

	// password is only sent when password_wo_version changed; otherwise nil
	// means the API leaves the password unchanged.
	if !data.PasswordWoVersion.Equal(state.PasswordWoVersion) {
		pw := data.PasswordWo.ValueString()
		body.Password = &pw
	}

	apiResp, err := r.client.UpdateUserWithResponse(ctx, int(data.Id.ValueInt64()), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating user", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Error updating user",
			fmt.Sprintf("User %d no longer exists; it was deleted outside of Terraform. The next plan will propose recreating it.", data.Id.ValueInt64()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating user", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenUser(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *maasUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data maasUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &maasclientv3.DeleteUserParams{}
	if !data.TransferResourcesToWo.IsNull() && !data.TransferResourcesToWo.IsUnknown() {
		to := int(data.TransferResourcesToWo.ValueInt64())
		params.TransferResourcesTo = &to
	}

	apiResp, err := r.client.DeleteUserWithResponse(ctx, int(data.Id.ValueInt64()), params)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting user", err.Error())
		return
	}
	// 204 = deleted; 404 = already gone. Both are success.
	if apiResp.StatusCode() != 204 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Error deleting user", apiError(apiResp.Status(), apiResp.Body))
	}
}

func flattenUser(user *maasclientv3.UserResponse, data *maasUserResourceModel) {
	data.Id = types.Int64Value(int64(user.Id))
	data.Username = types.StringValue(user.Username)
	data.FirstName = types.StringValue(user.FirstName)
	if user.LastName != nil {
		data.LastName = types.StringValue(*user.LastName)
	} else {
		// Not nullable in MAAS; nil here is loose API typing over a ""-canonical field.
		data.LastName = types.StringValue("")
	}
	if user.Email != nil {
		data.Email = types.StringValue(*user.Email)
	} else {
		// Can be null in MAAS; nil is a real, meaningful value.
		data.Email = types.StringNull()
	}
	// groups is always present in the response; flatten into the existing list.
	data.Groups, _ = types.ListValueFrom(context.Background(), types.Int64Type, groupIdsFromResponse(user.Groups))
}

func groupIdsFromResponse(groups []maasclientv3.UserGroupSummaryResponse) []int64 {
	ids := make([]int64, 0, len(groups))
	for _, g := range groups {
		ids = append(ids, int64(g.Id))
	}
	return ids
}

func optionalInt64List(l types.List) *[]int {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	elements := l.Elements()
	ids := make([]int, 0, len(elements))
	for _, e := range elements {
		v, ok := e.(types.Int64)
		if !ok {
			continue
		}
		ids = append(ids, int(v.ValueInt64()))
	}
	return &ids
}
