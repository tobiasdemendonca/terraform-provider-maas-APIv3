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

var _ resource.Resource = (*zoneResource)(nil)
var _ resource.ResourceWithImportState = (*zoneResource)(nil)

func NewZoneResource() resource.Resource {
	return &zoneResource{}
}

type zoneResource struct {
	client *maasclientv3.ClientWithResponses
}

type zoneResourceModel struct {
	Description types.String `tfsdk:"description"`
	Id          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
}

func (r *zoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (r *zoneResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a MAAS physical zone, a logical grouping of nodes.",
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The description of the zone.",
				Default:             stringdefault.StaticString(""), // Not nullable in MAAS.
			},
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The zone ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique name of the zone.",
			},
		},
	}
}

func (r *zoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Error importing zone", fmt.Sprintf("Invalid zone ID %q: %s", req.ID, err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.Int64Value(id))...)
}

func (r *zoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *zoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data zoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.CreateZoneJSONRequestBody{
		Name:        data.Name.ValueString(),
		Description: optionalString(data.Description),
	}

	apiResp, err := r.client.CreateZoneWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating zone", err.Error())
		return
	}

	if apiResp.StatusCode() == 409 {
		resp.Diagnostics.AddError("Error creating zone",
			fmt.Sprintf("A zone with the name %q already exists. Use a different name or import the existing zone", data.Name.ValueString()))
		return
	}

	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating zone", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenZone(apiResp.JSON201, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *zoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data zoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetZoneWithResponse(ctx, int(data.Id.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading zone", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading zone", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenZone(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *zoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data zoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.UpdateZoneJSONRequestBody{
		Name:        data.Name.ValueString(),
		Description: optionalString(data.Description),
	}

	apiResp, err := r.client.UpdateZoneWithResponse(ctx, int(data.Id.ValueInt64()), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating zone", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Error updating zone",
			fmt.Sprintf("Zone %d no longer exists; it was deleted outside of Terraform. The next plan will propose recreating it.", data.Id.ValueInt64()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating zone", apiError(apiResp.Status(), apiResp.Body))
		return
	}

	flattenZone(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *zoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data zoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteZoneWithResponse(ctx, int(data.Id.ValueInt64()), &maasclientv3.DeleteZoneParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting zone", err.Error())
		return
	}
	// 204 = deleted; 404 = already gone. 400 (e.g. default zone) is surfaced verbatim.
	if apiResp.StatusCode() != 204 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Error deleting zone", apiError(apiResp.Status(), apiResp.Body))
	}
}

// flattenZone maps the API response into the state model. Response fields are
// plain non-pointer types, so no nil handling is needed.
func flattenZone(zone *maasclientv3.ZoneResponse, data *zoneResourceModel) {
	data.Id = types.Int64Value(int64(zone.Id))
	data.Name = types.StringValue(zone.Name)
	data.Description = types.StringValue(zone.Description)
}
