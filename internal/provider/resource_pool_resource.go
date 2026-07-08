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

var _ resource.Resource = (*resourcePoolResource)(nil)
var _ resource.ResourceWithImportState = (*resourcePoolResource)(nil)

func NewResourcePoolResource() resource.Resource {
	return &resourcePoolResource{}
}

type resourcePoolResource struct {
	client *maasclientv3.ClientWithResponses
}

type resourcePoolResourceModel struct {
	Description types.String `tfsdk:"description"`
	Id          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
}

func (r *resourcePoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_pool"
}

func (r *resourcePoolResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a MAAS resource pool, a logical grouping of machines.",
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The description of the resource pool.",
				Default:             stringdefault.StaticString(""), // Not nullable in MAAS.
			},
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The resource pool ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique name of the resource pool.",
			},
		},
	}
}

func (r *resourcePoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Error importing resource pool", fmt.Sprintf("Invalid resource pool ID %q: %s", req.ID, err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.Int64Value(id))...)
}

func (r *resourcePoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *resourcePoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data resourcePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.CreateResourcePoolJSONRequestBody{
		Name:        data.Name.ValueString(),
		Description: optionalString(data.Description),
	}

	apiResp, err := r.client.CreateResourcePoolWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating resource pool", err.Error())
		return
	}

	if apiResp.StatusCode() == 409 {
		resp.Diagnostics.AddError("Error creating resource pool",
			fmt.Sprintf("A resource pool with the name %q already exists. Use a different name or import the existing resource pool", data.Name.ValueString()))
		return
	}

	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating resource pool", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenResourcePool(apiResp.JSON201, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *resourcePoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data resourcePoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetResourcePoolWithResponse(ctx, int(data.Id.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading resource pool", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading resource pool", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenResourcePool(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *resourcePoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data resourcePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.UpdateResourcePoolJSONRequestBody{
		Name:        data.Name.ValueString(),
		Description: optionalString(data.Description),
	}

	apiResp, err := r.client.UpdateResourcePoolWithResponse(ctx, int(data.Id.ValueInt64()), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating resource pool", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Error updating resource pool",
			fmt.Sprintf("Resource pool %d no longer exists; it was deleted outside of Terraform. The next plan will propose recreating it.", data.Id.ValueInt64()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating resource pool", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenResourcePool(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *resourcePoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data resourcePoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteResourcePoolWithResponse(ctx, int(data.Id.ValueInt64()), &maasclientv3.DeleteResourcePoolParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting resource pool", err.Error())
		return
	}
	// 204 = deleted; 404 = already gone. 400 (e.g. default pool) is surfaced verbatim.
	if apiResp.StatusCode() != 204 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Error deleting resource pool", apiError(apiResp.Status(), apiResp.Body))
	}
}

// flattenResourcePool maps the API response into the state model. Response
// fields are plain non-pointer types, so no nil handling is needed.
func flattenResourcePool(pool *maasclientv3.ResourcePoolResponse, data *resourcePoolResourceModel) {
	data.Id = types.Int64Value(int64(pool.Id))
	data.Name = types.StringValue(pool.Name)
	data.Description = types.StringValue(pool.Description)
}
