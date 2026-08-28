package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func init() {
	RegisterDataSource(NewIntegrationsDataSource)
}

var (
	_ datasource.DataSource              = (*integrationsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*integrationsDataSource)(nil)
)

// NewIntegrationsDataSource returns the cycle_integrations data source.
func NewIntegrationsDataSource() datasource.DataSource {
	return &integrationsDataSource{}
}

type integrationsDataSource struct {
	client *CycleClient
}

type integrationsDataSourceModel struct {
	Integrations []integrationModel `tfsdk:"integrations"`
}

func (d *integrationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integrations"
}

func (d *integrationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Cycle hub integrations.",
		Attributes: map[string]schema.Attribute{
			"integrations": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All integrations in the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: integrationDataSourceAttributes(false),
				},
			},
		},
	}
}

func (d *integrationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *integrationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config integrationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integrations, err := fetchAllIntegrations(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing integrations", err.Error())
		return
	}

	config.Integrations = make([]integrationModel, 0, len(integrations))
	for _, integ := range integrations {
		var item integrationModel
		resp.Diagnostics.Append(integrationModelFromAPI(ctx, &item, integ)...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.Integrations = append(config.Integrations, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// fetchAllIntegrations pages through GET /v1/integrations and returns every
// integration in the hub.
func fetchAllIntegrations(ctx context.Context, client *CycleClient) ([]cycle.Integration, error) {
	const pageSize = 100

	var all []cycle.Integration
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetIntegrationsWithResponse(ctx, &cycle.GetIntegrationsParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing integrations", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
