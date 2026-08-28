package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewServersDataSource)
}

var (
	_ datasource.DataSource              = (*serversDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serversDataSource)(nil)
)

// NewServersDataSource returns the cycle_servers data source.
func NewServersDataSource() datasource.DataSource {
	return &serversDataSource{}
}

type serversDataSource struct {
	client *CycleClient
}

type serversDataSourceModel struct {
	Cluster types.String                 `tfsdk:"cluster"`
	Servers []serversDataSourceItemModel `tfsdk:"servers"`
}

type serversDataSourceItemModel struct {
	ID         types.String `tfsdk:"id"`
	Hostname   types.String `tfsdk:"hostname"`
	Cluster    types.String `tfsdk:"cluster"`
	State      types.String `tfsdk:"state"`
	LocationID types.String `tfsdk:"location_id"`
	ModelID    types.String `tfsdk:"model_id"`
}

func (d *serversDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_servers"
}

func (d *serversDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Cycle servers in the hub, optionally filtered to a single cluster.",
		Attributes: map[string]schema.Attribute{
			"cluster": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "When set, only servers belonging to this cluster identifier are returned.",
			},
			"servers": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of servers.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the server.",
						},
						"hostname": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The hostname assigned to the server.",
						},
						"cluster": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The identifier of the cluster the server is deployed into.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the server.",
						},
						"location_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The provider location ID of the server.",
						},
						"model_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The provider server model ID.",
						},
					},
				},
			},
		},
	}
}

func (d *serversDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *serversDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config serversDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cluster *string
	if !config.Cluster.IsNull() && !config.Cluster.IsUnknown() {
		value := config.Cluster.ValueString()
		cluster = &value
	}

	servers, err := fetchAllServers(ctx, d.client, cluster)
	if err != nil {
		resp.Diagnostics.AddError("Error listing servers", err.Error())
		return
	}

	config.Servers = make([]serversDataSourceItemModel, 0, len(servers))
	for _, srv := range servers {
		config.Servers = append(config.Servers, serversDataSourceItemModel{
			ID:         types.StringValue(srv.Id),
			Hostname:   types.StringValue(srv.Hostname),
			Cluster:    types.StringValue(srv.Cluster),
			State:      types.StringValue(string(srv.State.Current)),
			LocationID: types.StringValue(srv.LocationId),
			ModelID:    types.StringValue(srv.ModelId),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// fetchAllServers pages through GET /v1/infrastructure/servers and returns
// every server, optionally filtered by cluster identifier.
func fetchAllServers(ctx context.Context, client *CycleClient, cluster *string) ([]cycle.Server, error) {
	const pageSize = 100

	var all []cycle.Server
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		params := &cycle.GetServersParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		}
		if cluster != nil && *cluster != "" {
			params.Filter = &struct {
				Cluster   *string `json:"cluster,omitempty"`
				Location  *string `json:"location,omitempty"`
				Providers *string `json:"providers,omitempty"`
				State     *string `json:"state,omitempty"`
				Tags      *string `json:"tags,omitempty"`
			}{Cluster: cluster}
		}

		listResp, err := client.Client.GetServersWithResponse(ctx, params)
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing servers", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
