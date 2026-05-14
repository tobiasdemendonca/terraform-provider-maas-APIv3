// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

var _ resource.Resource = (*sshKeyResource)(nil)

func NewSshKeyResource() resource.Resource {
	return &sshKeyResource{}
}

type sshKeyResource struct {
	client *maasclientv3.ClientWithResponses
}

type sshKeyResourceModel struct {
	Id  types.String `tfsdk:"id"`
	Key types.String `tfsdk:"key"`
}

func (r *sshKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (r *sshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an SSH public key for the current MAAS user.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Numeric SSH key ID, stored as a string.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The SSH public key string to upload.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *sshKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sshKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data sshKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := maasclientv3.CreateUserSshkeysJSONRequestBody{
		Key: data.Key.ValueString(),
	}

	apiResp, err := r.client.CreateUserSshkeysWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating SSH key", err.Error())
		return
	}
	if apiResp.JSON409 != nil {
		resp.Diagnostics.AddError("SSH key already exists",
			"An SSH key with this value already exists. Import it into Terraform state instead of creating a new resource.")
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating SSH key", fmt.Sprintf("API returned %s", apiResp.Status()))
		return
	}

	flattenSshKey(apiResp.JSON201, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *sshKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data sshKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid SSH key ID in state", err.Error())
		return
	}

	apiResp, err := r.client.GetUserSshkeyWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading SSH key", err.Error())
		return
	}
	if apiResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading SSH key", fmt.Sprintf("API returned %s", apiResp.Status()))
		return
	}

	flattenSshKey(apiResp.JSON200, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *sshKeyResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes have RequiresReplace; Terraform will never call Update.
}

func (r *sshKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data sshKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid SSH key ID in state", err.Error())
		return
	}

	apiResp, err := r.client.DeleteUserSshkeyWithResponse(ctx, id, &maasclientv3.DeleteUserSshkeyParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting SSH key", err.Error())
		return
	}
	if apiResp.StatusCode() != 204 && apiResp.StatusCode() != 404 {
		resp.Diagnostics.AddError("Error deleting SSH key", fmt.Sprintf("API returned %s", apiResp.Status()))
	}
}

func flattenSshKey(key *maasclientv3.SshKeyResponse, data *sshKeyResourceModel) {
	data.Id = types.StringValue(strconv.Itoa(key.Id))
	data.Key = types.StringValue(key.Key)
}
