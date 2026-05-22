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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
	"terraform-provider-maas-apiv3/internal/provider/resource_tag"
)

var _ resource.Resource = (*tagResource)(nil)

func NewTagResource() resource.Resource {
	return &tagResource{}
}

type tagResource struct {
	client *maasclientv3.ClientWithResponses
}

func (r *tagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (r *tagResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_tag.TagResourceSchema(ctx)
	s.MarkdownDescription = "Manages a MAAS tag."

	nameAttr := s.Attributes["name"].(schema.StringAttribute)
	nameAttr.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplace()}
	s.Attributes["name"] = nameAttr

	idAttr := s.Attributes["id"].(schema.Int64Attribute)
	idAttr.PlanModifiers = []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
	s.Attributes["id"] = idAttr

	resp.Schema = s
}

func (r *tagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data resource_tag.TagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.CreateTagJSONRequestBody{
		Name:       data.Name.ValueString(),
		Comment:    optionalString(data.Comment),
		Definition: optionalString(data.Definition),
		KernelOpts: optionalString(data.KernelOpts),
	}

	apiResp, err := r.client.CreateTagWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating tag", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating tag", fmt.Sprintf("API returned %s", apiResp.Status()))
		return
	}

	flattenTag(apiResp.JSON201, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *tagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data resource_tag.TagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetTagWithResponse(ctx, int(data.Id.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading tag", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading tag", fmt.Sprintf("API returned %s", apiResp.Status()))
		return
	}

	flattenTag(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *tagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data resource_tag.TagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.UpdateTagJSONRequestBody{
		Name:       data.Name.ValueString(),
		Comment:    optionalString(data.Comment),
		Definition: optionalString(data.Definition),
		KernelOpts: optionalString(data.KernelOpts),
	}

	apiResp, err := r.client.UpdateTagWithResponse(ctx, int(data.Id.ValueInt64()), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating tag", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating tag", fmt.Sprintf("API returned %s", apiResp.Status()))
		return
	}

	flattenTag(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *tagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data resource_tag.TagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.DeleteTagWithResponse(ctx, int(data.Id.ValueInt64()), &maasclientv3.DeleteTagParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting tag", err.Error())
		return
	}
	if apiResp.StatusCode() != 204 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Error deleting tag", fmt.Sprintf("API returned %s", apiResp.Status()))
	}
}

func flattenTag(tag *maasclientv3.TagResponse, data *resource_tag.TagModel) {
	data.Id = types.Int64Value(int64(tag.Id))
	data.Name = types.StringValue(tag.Name)
	data.Comment = types.StringValue(tag.Comment)
	data.Definition = types.StringValue(tag.Definition)
	data.KernelOpts = types.StringValue(tag.KernelOpts)
}

func optionalString(s types.String) *string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}
	v := s.ValueString()
	return &v
}
