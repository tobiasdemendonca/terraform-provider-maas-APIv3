// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

const (
	MaasURLEnvKey      = "TF_MAAS_URL"
	MaasUserEnvKey     = "TF_MAAS_USER"
	MaasPasswordEnvKey = "TF_MAAS_PWD"
)

var _ provider.Provider = &MaasProvider{}

// MaasProvider is the root Terraform provider for MAAS API v3.
type MaasProvider struct {
	version string
}

// MaasProviderModel is the decoded HCL/env configuration block.
type MaasProviderModel struct {
	URL      types.String `tfsdk:"url"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

// MaasProviderData is passed to every resource and data source via
// resp.ResourceData / resp.DataSourceData.
type MaasProviderData struct {
	Client *maasclientv3.ClientWithResponses
}

func (p *MaasProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "maas"
	resp.Version = p.version
}

func (p *MaasProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("MAAS API base URL (e.g. `http://10.0.0.1:5240`). Can also be set via `%s`.", MaasURLEnvKey),
				Optional:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("MAAS username. Can also be set via `%s`.", MaasUserEnvKey),
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("MAAS password. Can also be set via `%s`.", MaasPasswordEnvKey),
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *MaasProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config MaasProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url, username, password := resolveMaasConfig(config)

	if url == "" {
		resp.Diagnostics.AddError("Missing MAAS URL",
			fmt.Sprintf("Set url in the provider block or the %s environment variable.", MaasURLEnvKey))
	}
	if username == "" {
		resp.Diagnostics.AddError("Missing MAAS username",
			fmt.Sprintf("Set username in the provider block or the %s environment variable.", MaasUserEnvKey))
	}
	if password == "" {
		resp.Diagnostics.AddError("Missing MAAS password",
			fmt.Sprintf("Set password in the provider block or the %s environment variable.", MaasPasswordEnvKey))
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Plain client with no auth editor, used only for login calls inside tokenManager.
	authClient, err := maasclientv3.NewClientWithResponses(url)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create MAAS auth client", err.Error())
		return
	}

	tm := &tokenManager{
		serverURL:  url,
		username:   username,
		password:   password,
		authClient: authClient,
	}

	// Eagerly authenticate to validate credentials before any resource operations.
	if _, err := tm.ensureValidToken(ctx); err != nil {
		resp.Diagnostics.AddError("MAAS authentication failed", err.Error())
		return
	}

	client, err := maasclientv3.NewClientWithResponses(url,
		maasclientv3.WithRequestEditorFn(tm.requestEditor),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create MAAS client", err.Error())
		return
	}

	providerData := MaasProviderData{Client: client}
	resp.ResourceData = providerData
	resp.DataSourceData = providerData
}

func (p *MaasProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewFabricResource,
		NewResourcePoolResource,
		NewZoneResource,
		NewMaasUserResource,
	}
}

func (p *MaasProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &MaasProvider{version: version}
	}
}

// resolveMaasConfig returns url, username, password, preferring explicit
// provider config over environment variables.
func resolveMaasConfig(config MaasProviderModel) (url, username, password string) {
	url = os.Getenv(MaasURLEnvKey)
	if !config.URL.IsNull() && !config.URL.IsUnknown() {
		url = config.URL.ValueString()
	}
	username = os.Getenv(MaasUserEnvKey)
	if !config.Username.IsNull() && !config.Username.IsUnknown() {
		username = config.Username.ValueString()
	}
	password = os.Getenv(MaasPasswordEnvKey)
	if !config.Password.IsNull() && !config.Password.IsUnknown() {
		password = config.Password.ValueString()
	}
	return
}

// tokenManager handles JWT acquisition and proactive refresh for the MAAS client.
// It is safe for concurrent use.
type tokenManager struct {
	mu          sync.Mutex
	serverURL   string
	username    string
	password    string
	accessToken string
	expiresAt   time.Time
	authClient  *maasclientv3.ClientWithResponses
}

// requestEditor satisfies maasclientv3.RequestEditorFn. It is called by the
// generated client before every outbound request.
func (tm *tokenManager) requestEditor(ctx context.Context, req *http.Request) error {
	token, err := tm.ensureValidToken(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// ensureValidToken returns the current access token, refreshing it first if it
// is within 30 seconds of expiry.
func (tm *tokenManager) ensureValidToken(ctx context.Context) (string, error) {
	tm.mu.Lock()
	if tm.accessToken != "" && time.Now().Add(30*time.Second).Before(tm.expiresAt) {
		token := tm.accessToken
		tm.mu.Unlock()
		return token, nil
	}
	tm.mu.Unlock()

	return tm.login(ctx)
}

// login authenticates with MAAS and stores the new access token.
func (tm *tokenManager) login(ctx context.Context) (string, error) {
	resp, err := tm.authClient.LoginWithFormdataBodyWithResponse(ctx,
		maasclientv3.LoginFormdataRequestBody{
			Username: tm.username,
			Password: tm.password,
		},
	)
	if err != nil {
		return "", fmt.Errorf("MAAS login request failed: %w", err)
	}
	if resp.JSON200 == nil {
		return "", fmt.Errorf("MAAS login failed: %s", resp.Status())
	}

	token := resp.JSON200.AccessToken
	expiry := jwtExpiry(token)

	tm.mu.Lock()
	tm.accessToken = token
	tm.expiresAt = expiry
	tm.mu.Unlock()

	return token, nil
}

// jwtExpiry extracts the exp claim from a JWT without a library dependency.
// Falls back to 5 minutes from now if the token cannot be parsed.
func jwtExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Now().Add(5 * time.Minute)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Now().Add(5 * time.Minute)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Now().Add(5 * time.Minute)
	}
	return time.Unix(claims.Exp, 0)
}
