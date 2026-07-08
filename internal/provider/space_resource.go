package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

var _ resource.Resource = (*spaceResource)(nil)
var _ resource.ResourceWithImportState = (*spaceResource)(nil)

func NewSpaceResource() resource.Resource {
	return &spaceResource{}
}

// spaceResource implements the `maas_space` resource.
type spaceResource struct {
	client *maasclientv3.ClientWithResponses
}

// The Terraform state model for the space resource.
type spaceResourceModel struct {
	Description types.String `tfsdk:"description"`
	Id          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
}

func (r *spaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space"
}

func (r *spaceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a MAAS network space.",
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The description of the space.",
				Default:             stringdefault.StaticString(""), // Not nullable in MAAS, so default to "" to match the read value.
			},
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The space ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique name of the space.",
			},
		},
	}
}

func (r *spaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Error importing space", fmt.Sprintf("Invalid space ID %q: %s", req.ID, err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.Int64Value(id))...)
}

func (r *spaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *spaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data spaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.CreateSpaceJSONRequestBody{
		Name:        data.Name.ValueString(),
		Description: optionalString(data.Description),
	}

	apiResp, err := r.client.CreateSpaceWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating space", err.Error())
		return
	}

	if apiResp.StatusCode() == 409 {
		resp.Diagnostics.AddError("Error creating space",
			fmt.Sprintf("A space with the name %q already exists. Use a different name or import the existing space", data.Name.ValueString()))
		return
	}

	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating space", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenSpace(apiResp.JSON201, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *spaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data spaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetSpaceWithResponse(ctx, int(data.Id.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading space", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading space", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenSpace(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *spaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data spaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.UpdateSpaceJSONRequestBody{
		Name:        data.Name.ValueString(),
		Description: optionalString(data.Description),
	}

	apiResp, err := r.client.UpdateSpaceWithResponse(ctx, int(data.Id.ValueInt64()), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating space", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Error updating space",
			fmt.Sprintf("Space %d no longer exists; it was deleted outside of Terraform. The next plan will propose recreating it.", data.Id.ValueInt64()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating space", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenSpace(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *spaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data spaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteSpaceWithResponse(ctx, int(data.Id.ValueInt64()), &maasclientv3.DeleteSpaceParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting space", err.Error())
		return
	}
	// 204 = deleted; 404 = already gone. Both are success.
	if apiResp.StatusCode() != 204 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Error deleting space", apiError(apiResp.Status(), apiResp.Body))
	}
}

// flattenSpace maps the API response into the Terraform state model.
// See AGENTS.md (Unmarshal section): same response type (*string), different
// flatten per MAAS semantics. description is not nullable in MAAS, so nil is
// coerced to ""; name is defensively treated as nullable in the response type.
func flattenSpace(space *maasclientv3.SpaceResponse, data *spaceResourceModel) {
	data.Id = types.Int64Value(int64(space.Id))
	if space.Name != nil {
		data.Name = types.StringValue(*space.Name)
	} else {
		data.Name = types.StringNull()
	}
	if space.Description != nil {
		data.Description = types.StringValue(*space.Description)
	} else {
		// Not nullable in MAAS; nil here is loose API typing over a ""-canonical field.
		data.Description = types.StringValue("")
	}
}
