package provider

import (
	"context"
	"os"

	"github.com/Sovforge/Scutum/contrib/terraform/provider/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &ScutumProvider{}

type ScutumProvider struct{ version string }

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &ScutumProvider{version: version} }
}

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func (p *ScutumProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "scutum"
	resp.Version = p.version
}

func (p *ScutumProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Scutum WireGuard mesh resources as infrastructure-as-code.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "Scutum API base URL (e.g. https://scutum.example.com). Reads SCUTUM_ENDPOINT env var if omitted.",
				Optional:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "Scutum API key. Reads SCUTUM_API_KEY env var if omitted.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *ScutumProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := os.Getenv("SCUTUM_ENDPOINT")
	if !cfg.Endpoint.IsNull() && !cfg.Endpoint.IsUnknown() {
		endpoint = cfg.Endpoint.ValueString()
	}
	apiKey := os.Getenv("SCUTUM_API_KEY")
	if !cfg.APIKey.IsNull() && !cfg.APIKey.IsUnknown() {
		apiKey = cfg.APIKey.ValueString()
	}

	if endpoint == "" {
		resp.Diagnostics.AddError("Missing endpoint", "Set endpoint or SCUTUM_ENDPOINT")
		return
	}
	if apiKey == "" {
		resp.Diagnostics.AddError("Missing api_key", "Set api_key or SCUTUM_API_KEY")
		return
	}

	c := client.New(endpoint, apiKey)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *ScutumProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNodeResource,
		NewGroupResource,
		NewFederationPeerResource,
		NewWebhookResource,
		NewUserResource,
	}
}

func (p *ScutumProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}