// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

// Ensure the implementation satisfies the expected interfaces.
var _ resource.Resource = (*fabricResource)(nil)

func NewFabricResource() resource.Resource {
	return &fabricResource{}
}

// fabricResource implements the `maas_fabric` resource.
type fabricResource struct {
	client *maasclientv3.ClientWithResponses
}

// fabricResourceModel is the Terraform state model for a MAAS fabric.
//
// Field nullability mirrors what MAAS *stores* for "absent", not what the
// API request accepts (see the tag resource for the full rule):
//   - description: DB column NOT NULL, canonical absent value is "". Modeled
//     with a schema Default("") so null/omitted normalizes to "" and the
//     field can be cleared back to "". Computed is required by Default.
//   - class_type: DB column null=True; NULL is a real, storable value.
//     Modeled Optional-only so null round-trips and the field can be cleared
//     to null. No Computed, no Default (Computed without a Default would trap
//     the prior value and prevent clearing).
type fabricResourceModel struct {
	ClassType   types.String `tfsdk:"class_type"`
	Description types.String `tfsdk:"description"`
	Id          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
}

func (r *fabricResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_fabric"
}

func (r *fabricResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a MAAS fabric.",
		Attributes: map[string]schema.Attribute{
			"class_type": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Optional class type for the fabric. Omit or set to null to clear. " +
					"Unlike `description`, this field is genuinely nullable: MAAS stores NULL when absent.",
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "The description of the fabric. Defaults to an empty string. " +
					"The MAAS DB stores \"\" for absent (NOT NULL), so null normalizes to \"\".",
				Default: stringdefault.StaticString(""),
			},
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The fabric ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The unique name of the fabric. The name may be changed in place; " +
					"MAAS does not require replacement (the update endpoint is keyed by ID).",
			},
		},
	}
}

func (r *fabricResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *fabricResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data fabricResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.CreateFabricJSONRequestBody{
		Name:        data.Name.ValueString(),
		ClassType:   optionalString(data.ClassType),
		Description: optionalString(data.Description),
	}

	apiResp, err := r.client.CreateFabricWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating fabric", err.Error())
		return
	}
	// A 409 means a fabric with this name already exists. Practitioners should
	// import the existing resource rather than letting Terraform recreate it.
	if apiResp.StatusCode() == 409 {
		resp.Diagnostics.AddError("Error creating fabric",
			fmt.Sprintf("A fabric with the name %q already exists. Import the existing fabric "+
				"with `terraform import <address> <fabric_id>` instead of creating it.",
				data.Name.ValueString()))
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating fabric", fmt.Sprintf("API returned %s", apiResp.Status()))
		return
	}

	flattenFabric(apiResp.JSON201, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *fabricResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data fabricResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetFabricWithResponse(ctx, int(data.Id.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading fabric", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading fabric", fmt.Sprintf("API returned %s", apiResp.Status()))
		return
	}

	flattenFabric(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *fabricResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data fabricResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.UpdateFabricJSONRequestBody{
		Name:        data.Name.ValueString(),
		ClassType:   optionalString(data.ClassType),
		Description: optionalString(data.Description),
	}

	apiResp, err := r.client.UpdateFabricWithResponse(ctx, int(data.Id.ValueInt64()), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating fabric", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating fabric", fmt.Sprintf("API returned %s", apiResp.Status()))
		return
	}

	flattenFabric(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *fabricResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data fabricResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteFabricWithResponse(ctx, int(data.Id.ValueInt64()), &maasclientv3.DeleteFabricParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting fabric", err.Error())
		return
	}
	// 204 = deleted; 404 = already gone. Both are success.
	if apiResp.StatusCode() != 204 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Error deleting fabric", fmt.Sprintf("API returned %s", apiResp.Status()))
	}
}

// flattenFabric maps the API response into the Terraform state model.
//
// The response type (*string) is the same for description and class_type, but
// they flatten differently because their DB semantics differ:
//   - description: DB NOT NULL, "" canonical -> a nil response (loose typing
//     in the API model) is coerced to "" so state matches the canonical form.
//   - class_type: DB null=True, NULL is meaningful -> a nil response is
//     preserved as null in state so it round-trips and can be cleared.
func flattenFabric(fabric *maasclientv3.FabricResponse, data *fabricResourceModel) {
	data.Id = types.Int64Value(int64(fabric.Id))
	if fabric.Name != nil {
		data.Name = types.StringValue(*fabric.Name)
	} else {
		data.Name = types.StringNull()
	}
	if fabric.Description != nil {
		data.Description = types.StringValue(*fabric.Description)
	} else {
		// DB is NOT NULL; nil here is loose API typing over a ""-canonical field.
		data.Description = types.StringValue("")
	}
	if fabric.ClassType != nil {
		data.ClassType = types.StringValue(*fabric.ClassType)
	} else {
		// DB allows NULL; nil is a real, meaningful value.
		data.ClassType = types.StringNull()
	}
}
