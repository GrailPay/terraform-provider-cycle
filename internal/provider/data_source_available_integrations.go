package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewAvailableIntegrationsDataSource)
}

var (
	_ datasource.DataSource              = (*availableIntegrationsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*availableIntegrationsDataSource)(nil)
)

// NewAvailableIntegrationsDataSource returns the cycle_available_integrations data source.
func NewAvailableIntegrationsDataSource() datasource.DataSource {
	return &availableIntegrationsDataSource{}
}

type availableIntegrationsDataSource struct {
	client *CycleClient
}

type availableIntegrationsDataSourceModel struct {
	Integrations []availableIntegrationModel `tfsdk:"integrations"`
}

type availableIntegrationModel struct {
	Category             types.String `tfsdk:"category"`
	Vendor               types.String `tfsdk:"vendor"`
	Name                 types.String `tfsdk:"name"`
	URL                  types.String `tfsdk:"url"`
	Usable               types.Bool   `tfsdk:"usable"`
	Editable             types.Bool   `tfsdk:"editable"`
	Public               types.Bool   `tfsdk:"public"`
	SupportsMultiple     types.Bool   `tfsdk:"supports_multiple"`
	SupportsVerification types.Bool   `tfsdk:"supports_verification"`
	Deprecated           types.Bool   `tfsdk:"deprecated"`
	Features             types.List   `tfsdk:"features"`
	Extends              types.List   `tfsdk:"extends"`
}

func (d *availableIntegrationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_available_integrations"
}

func (d *availableIntegrationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists integration vendors the current hub can enable, flattened from the categorized " +
			"`GET /v1/integrations/available` response.",
		Attributes: map[string]schema.Attribute{
			"integrations": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Available integration definitions across all categories.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"category": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The catalog category: `billing`, `image-builders`, `infrastructure-provider`, `object-storage`, or `tls-certificate-generation`.",
						},
						"vendor": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The vendor identifier used when creating a `cycle_integration`.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The human-readable name of the integration.",
						},
						"url": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "A URL with more information about the integration.",
						},
						"usable": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this integration can be used at this time.",
						},
						"editable": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether an existing instance of this integration can be edited.",
						},
						"public": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this integration is publicly available.",
						},
						"supports_multiple": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the hub can create more than one instance of this integration.",
						},
						"supports_verification": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this integration supports a verification job after create.",
						},
						"deprecated": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this integration is deprecated. `false` when the API omits the field.",
						},
						"features": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Additional features supported by this integration. Empty when omitted.",
						},
						"extends": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Functionality this integration extends (e.g. `backups`). Empty when omitted.",
						},
					},
				},
			},
		},
	}
}

func (d *availableIntegrationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *availableIntegrationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config availableIntegrationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Client.GetAvailableIntegrationsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing available integrations", err.Error())
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "listing available integrations", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	data := getResp.JSON200.Data
	categories := []struct {
		name string
		defs *[]cycle.IntegrationDefinition
	}{
		{"billing", data.Billing},
		{"image-builders", data.ImageBuilders},
		{"infrastructure-provider", data.InfrastructureProvider},
		{"object-storage", data.ObjectStorage},
		{"tls-certificate-generation", data.TlsCertificateGeneration},
	}

	config.Integrations = make([]availableIntegrationModel, 0)
	for _, cat := range categories {
		if cat.defs == nil {
			continue
		}
		for _, def := range *cat.defs {
			item, d := availableIntegrationFromAPI(ctx, cat.name, def)
			resp.Diagnostics.Append(d...)
			if resp.Diagnostics.HasError() {
				return
			}
			config.Integrations = append(config.Integrations, item)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func availableIntegrationFromAPI(ctx context.Context, category string, def cycle.IntegrationDefinition) (availableIntegrationModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	features, d := stringListFromAPI(ctx, def.Features)
	diags.Append(d...)
	if diags.HasError() {
		return availableIntegrationModel{}, diags
	}
	extends, d := stringListFromAPI(ctx, def.Extends)
	diags.Append(d...)
	if diags.HasError() {
		return availableIntegrationModel{}, diags
	}

	deprecated := false
	if def.Deprecated != nil {
		deprecated = *def.Deprecated
	}

	return availableIntegrationModel{
		Category:             types.StringValue(category),
		Vendor:               types.StringValue(def.Vendor),
		Name:                 types.StringValue(def.Name),
		URL:                  types.StringValue(def.Url),
		Usable:               types.BoolValue(def.Usable),
		Editable:             types.BoolValue(def.Editable),
		Public:               types.BoolValue(def.Public),
		SupportsMultiple:     types.BoolValue(def.SupportsMultiple),
		SupportsVerification: types.BoolValue(def.SupportsVerification),
		Deprecated:           types.BoolValue(deprecated),
		Features:             features,
		Extends:              extends,
	}, diags
}

func stringListFromAPI(ctx context.Context, in *[]string) (types.List, diag.Diagnostics) {
	if in == nil {
		return types.ListValueFrom(ctx, types.StringType, []string{})
	}
	return types.ListValueFrom(ctx, types.StringType, *in)
}
