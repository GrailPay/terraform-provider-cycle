package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewClustersDataSource)
}

var (
	_ datasource.DataSource              = (*clustersDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*clustersDataSource)(nil)
)

// NewClustersDataSource returns the cycle_clusters data source.
func NewClustersDataSource() datasource.DataSource {
	return &clustersDataSource{}
}

type clustersDataSource struct {
	client *CycleClient
}

type clustersDataSourceModel struct {
	Clusters []clusterDataSourceModel `tfsdk:"clusters"`
}

func (d *clustersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_clusters"
}

func (d *clustersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Cycle infrastructure clusters in the hub.",
		Attributes: map[string]schema.Attribute{
			"clusters": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The clusters in the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the cluster.",
						},
						"identifier": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The human-readable slugged identifier of the cluster.",
						},
						"non_essential": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the cluster is marked as non-essential.",
						},
						"hub_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the hub this cluster belongs to.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the cluster.",
						},
					},
				},
			},
		},
	}
}

func (d *clustersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *clustersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config clustersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusters, err := fetchAllClusters(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing clusters", err.Error())
		return
	}

	config.Clusters = make([]clusterDataSourceModel, 0, len(clusters))
	for _, cluster := range clusters {
		config.Clusters = append(config.Clusters, clusterDataSourceModel{
			ID:           types.StringValue(cluster.Id),
			Identifier:   types.StringValue(cluster.Identifier),
			NonEssential: types.BoolValue(cluster.NonEssential),
			HubID:        types.StringValue(cluster.HubId),
			State:        types.StringValue(string(cluster.State.Current)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
