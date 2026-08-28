package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewIPPoolsDataSource)
}

var (
	_ datasource.DataSource              = (*ipPoolsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*ipPoolsDataSource)(nil)
)

// NewIPPoolsDataSource returns the cycle_ip_pools data source.
func NewIPPoolsDataSource() datasource.DataSource {
	return &ipPoolsDataSource{}
}

type ipPoolsDataSource struct {
	client *CycleClient
}

type ipPoolsDataSourceModel struct {
	IPPools []ipPoolDataModel `tfsdk:"ip_pools"`
}

type ipPoolDataModel struct {
	ID           types.String `tfsdk:"id"`
	CIDR         types.String `tfsdk:"cidr"`
	Gateway      types.String `tfsdk:"gateway"`
	Kind         types.String `tfsdk:"kind"`
	Floating     types.Bool   `tfsdk:"floating"`
	LocationID   types.String `tfsdk:"location_id"`
	ServerID     types.String `tfsdk:"server_id"`
	IPsAvailable types.Int64  `tfsdk:"ips_available"`
	IPsTotal     types.Int64  `tfsdk:"ips_total"`
	State        types.String `tfsdk:"state"`
}

func (d *ipPoolsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_pools"
}

func (d *ipPoolsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Cycle IP pools in the hub. The API reports available/total IP counts only; " +
			"it does not enumerate individual addresses.",
		Attributes: map[string]schema.Attribute{
			"ip_pools": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of IP pools.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: ipPoolDataAttributes(),
				},
			},
		},
	}
}

func ipPoolDataAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The unique ID of the IP pool.",
		},
		"cidr": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The CIDR block for the pool.",
		},
		"gateway": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The gateway address for the pool.",
		},
		"kind": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The type of IP pool.",
		},
		"floating": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether this is a floating IP pool.",
		},
		"location_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The location ID associated with the pool.",
		},
		"server_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The server ID associated with the pool, if any.",
		},
		"ips_available": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of IPs in the pool that are still available to assign. The API does not list the individual addresses.",
		},
		"ips_total": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Total number of IPs in the pool.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The current state of the pool.",
		},
	}
}

func (d *ipPoolsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *ipPoolsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ipPoolsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pools, err := fetchAllIPPools(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing IP pools", err.Error())
		return
	}

	config.IPPools = make([]ipPoolDataModel, 0, len(pools))
	for _, pool := range pools {
		config.IPPools = append(config.IPPools, ipPoolDataModelFromAPI(pool))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func ipPoolDataModelFromAPI(pool cycle.IpPool) ipPoolDataModel {
	return ipPoolDataModel{
		ID:           types.StringValue(pool.Id),
		CIDR:         types.StringValue(pool.Block.Cidr),
		Gateway:      types.StringValue(pool.Block.Gateway),
		Kind:         types.StringValue(string(pool.Kind)),
		Floating:     types.BoolValue(pool.Floating),
		LocationID:   types.StringValue(pool.LocationId),
		ServerID:     types.StringValue(pool.ServerId),
		IPsAvailable: types.Int64Value(int64(pool.Ips.Available)),
		IPsTotal:     types.Int64Value(int64(pool.Ips.Total)),
		State:        types.StringValue(string(pool.State.Current)),
	}
}

// fetchAllIPPools pages through GET /v1/infrastructure/ips/pools.
func fetchAllIPPools(ctx context.Context, client *CycleClient) ([]cycle.IpPool, error) {
	const pageSize = 100

	var all []cycle.IpPool
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetIpPoolsWithResponse(ctx, &cycle.GetIpPoolsParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing IP pools", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
