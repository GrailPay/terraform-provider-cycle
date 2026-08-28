package provider

import (
	"context"
	"os"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultAPIURL = "https://api.cycle.io"

var _ provider.Provider = (*CycleProvider)(nil)

// New returns a provider factory for providerserver.Serve and tests.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CycleProvider{version: version}
	}
}

// CycleProvider implements the Terraform provider for cycle.io.
type CycleProvider struct {
	version string
}

type cycleProviderModel struct {
	APIKey types.String `tfsdk:"api_key"`
	HubID  types.String `tfsdk:"hub_id"`
	APIURL types.String `tfsdk:"api_url"`
}

func (p *CycleProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "cycle"
	resp.Version = p.version
}

func (p *CycleProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with the cycle.io platform.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Cycle API key. May also be provided via the CYCLE_API_KEY environment variable.",
			},
			"hub_id": schema.StringAttribute{
				Optional:    true,
				Description: "ID of the Cycle hub to manage. May also be provided via the CYCLE_HUB_ID environment variable.",
			},
			"api_url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of the Cycle API. Defaults to " + defaultAPIURL + ".",
			},
		},
	}
}

func (p *CycleProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config cycleProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown Cycle API Key",
			"The provider cannot connect to the Cycle API because api_key is derived from a value that is not yet known. "+
				"Set a static value, or use the CYCLE_API_KEY environment variable.",
		)
	}
	if config.HubID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("hub_id"),
			"Unknown Cycle Hub ID",
			"The provider cannot connect to the Cycle API because hub_id is derived from a value that is not yet known. "+
				"Set a static value, or use the CYCLE_HUB_ID environment variable.",
		)
	}
	if config.APIURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Unknown Cycle API URL",
			"The provider cannot connect to the Cycle API because api_url is derived from a value that is not yet known.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("CYCLE_API_KEY")
	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	}

	hubID := os.Getenv("CYCLE_HUB_ID")
	if !config.HubID.IsNull() {
		hubID = config.HubID.ValueString()
	}

	apiURL := defaultAPIURL
	if !config.APIURL.IsNull() {
		apiURL = config.APIURL.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Cycle API Key",
			"Set the api_key attribute in the provider configuration or the CYCLE_API_KEY environment variable.",
		)
	}
	if hubID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("hub_id"),
			"Missing Cycle Hub ID",
			"Set the hub_id attribute in the provider configuration or the CYCLE_HUB_ID environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiClient, err := cycle.NewAuthenticatedClient(cycle.ClientConfig{
		APIKey:  apiKey,
		HubID:   hubID,
		BaseURL: &apiURL,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Cycle API Client",
			"An unexpected error occurred creating the Cycle API client: "+err.Error(),
		)
		return
	}

	client := &CycleClient{
		Client: apiClient,
		HubID:  hubID,
	}
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *CycleProvider) Resources(_ context.Context) []func() resource.Resource {
	return resourceFactories
}

func (p *CycleProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return dataSourceFactories
}
