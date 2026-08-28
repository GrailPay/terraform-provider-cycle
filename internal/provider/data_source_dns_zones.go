package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewDnsZonesDataSource)
}

var (
	_ datasource.DataSource              = (*dnsZonesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dnsZonesDataSource)(nil)
)

// NewDnsZonesDataSource returns the cycle_dns_zones data source.
func NewDnsZonesDataSource() datasource.DataSource {
	return &dnsZonesDataSource{}
}

type dnsZonesDataSource struct {
	client *CycleClient
}

type dnsZonesDataSourceModel struct {
	Zones []dnsZoneDataSourceModel `tfsdk:"zones"`
}

func (d *dnsZonesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zones"
}

func (d *dnsZonesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Cycle DNS zones in the hub.",
		Attributes: map[string]schema.Attribute{
			"zones": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The DNS zones in the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the DNS zone.",
						},
						"origin": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The origin (domain name) of the DNS zone, e.g. `example.com`.",
						},
						"hosted": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this zone is hosted by Cycle (`true`) or linked (`false`).",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the DNS zone.",
						},
					},
				},
			},
		},
	}
}

func (d *dnsZonesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *dnsZonesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config dnsZonesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zones, err := fetchAllDNSZones(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing DNS zones", err.Error())
		return
	}

	config.Zones = make([]dnsZoneDataSourceModel, 0, len(zones))
	for _, zone := range zones {
		config.Zones = append(config.Zones, dnsZoneDataSourceModel{
			ID:     types.StringValue(zone.Id),
			Origin: types.StringValue(zone.Origin),
			Hosted: types.BoolValue(zone.Hosted),
			State:  types.StringValue(string(zone.State.Current)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func fetchAllDNSZones(ctx context.Context, client *CycleClient) ([]cycle.DnsZone, error) {
	const pageSize = 100

	var all []cycle.DnsZone
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		apiResp, err := client.Client.GetDNSZonesWithResponse(ctx, &cycle.GetDNSZonesParams{
			Page: &cycle.PageParam{Number: &number, Size: &size},
		})
		if err != nil {
			return nil, err
		}
		if apiResp.JSON200 == nil {
			return nil, apiError("listing DNS zones", apiResp.StatusCode(), apiResp.JSONDefault)
		}

		all = append(all, apiResp.JSON200.Data...)
		if len(apiResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
