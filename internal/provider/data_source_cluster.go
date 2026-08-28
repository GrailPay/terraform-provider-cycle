package provider

import (
	"context"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewClusterDataSource)
}

var (
	_ datasource.DataSource                     = (*clusterDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*clusterDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*clusterDataSource)(nil)
)

// NewClusterDataSource returns the cycle_cluster data source.
func NewClusterDataSource() datasource.DataSource {
	return &clusterDataSource{}
}

type clusterDataSource struct {
	client *CycleClient
}

type clusterDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Identifier   types.String `tfsdk:"identifier"`
	NonEssential types.Bool   `tfsdk:"non_essential"`
	HubID        types.String `tfsdk:"hub_id"`
	State        types.String `tfsdk:"state"`
}

func (d *clusterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (d *clusterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle infrastructure cluster by ID or identifier.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique ID of the cluster. Exactly one of `id` or `identifier` must be set.",
			},
			"identifier": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The human-readable slugged identifier of the cluster. Exactly one of `id` or `identifier` must be set.",
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
	}
}

func (d *clusterDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("identifier"),
		),
	}
}

func (d *clusterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *clusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config clusterDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cluster cycle.Cluster
	if !config.ID.IsNull() {
		getResp, err := d.client.Client.GetClusterWithResponse(ctx, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading cluster", err.Error())
			return
		}
		if getResp.StatusCode() == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Cluster Not Found",
				fmt.Sprintf("No cluster with ID %q exists in this hub.", config.ID.ValueString()),
			)
			return
		}
		if getResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "reading cluster", getResp.StatusCode(), getResp.JSONDefault)
			return
		}
		cluster = getResp.JSON200.Data
	} else {
		clusters, err := fetchAllClusters(ctx, d.client)
		if err != nil {
			resp.Diagnostics.AddError("Error listing clusters", err.Error())
			return
		}

		identifier := config.Identifier.ValueString()
		var matches []cycle.Cluster
		for _, c := range clusters {
			if c.Identifier == identifier {
				matches = append(matches, c)
			}
		}
		switch len(matches) {
		case 0:
			resp.Diagnostics.AddError(
				"Cluster Not Found",
				fmt.Sprintf("No cluster with identifier %q exists in this hub.", identifier),
			)
			return
		case 1:
			cluster = matches[0]
		default:
			resp.Diagnostics.AddError(
				"Multiple Clusters Found",
				fmt.Sprintf("Found %d clusters with identifier %q. Cluster identifiers are not required to be unique; use the id attribute instead.", len(matches), identifier),
			)
			return
		}
	}

	config.ID = types.StringValue(cluster.Id)
	config.Identifier = types.StringValue(cluster.Identifier)
	config.NonEssential = types.BoolValue(cluster.NonEssential)
	config.HubID = types.StringValue(cluster.HubId)
	config.State = types.StringValue(string(cluster.State.Current))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// fetchAllClusters pages through GET /v1/infrastructure/clusters and returns
// every cluster in the hub.
func fetchAllClusters(ctx context.Context, client *CycleClient) ([]cycle.Cluster, error) {
	const pageSize = 100

	var all []cycle.Cluster
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetClustersWithResponse(ctx, &cycle.GetClustersParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing clusters", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
