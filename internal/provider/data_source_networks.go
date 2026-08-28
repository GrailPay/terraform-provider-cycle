package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func init() {
	RegisterDataSource(NewNetworksDataSource)
}

var (
	_ datasource.DataSource              = (*networksDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*networksDataSource)(nil)
)

// NewNetworksDataSource returns the cycle_networks data source.
func NewNetworksDataSource() datasource.DataSource {
	return &networksDataSource{}
}

type networksDataSource struct {
	client *CycleClient
}

type networksDataSourceModel struct {
	Networks []networkModel `tfsdk:"networks"`
}

func (d *networksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_networks"
}

func (d *networksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Cycle SDN networks in the hub.",
		Attributes: map[string]schema.Attribute{
			"networks": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All SDN networks in the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: networkDataSourceAttributes(false),
				},
			},
		},
	}
}

func (d *networksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *networksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config networksDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	networks, err := fetchAllNetworks(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing networks", err.Error())
		return
	}

	config.Networks = make([]networkModel, 0, len(networks))
	for _, net := range networks {
		var item networkModel
		resp.Diagnostics.Append(networkModelFromAPI(ctx, &item, net, true)...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.Networks = append(config.Networks, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// fetchAllNetworks pages through GET /v1/sdn/networks and returns every
// network in the hub.
func fetchAllNetworks(ctx context.Context, client *CycleClient) ([]cycle.Network, error) {
	const pageSize = 100

	var all []cycle.Network
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetNetworksWithResponse(ctx, &cycle.GetNetworksParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing networks", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
