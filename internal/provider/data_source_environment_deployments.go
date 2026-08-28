package provider

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewEnvironmentDeploymentsDataSource)
}

var (
	_ datasource.DataSource              = (*environmentDeploymentsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*environmentDeploymentsDataSource)(nil)
)

// NewEnvironmentDeploymentsDataSource returns the cycle_environment_deployments data source.
func NewEnvironmentDeploymentsDataSource() datasource.DataSource {
	return &environmentDeploymentsDataSource{}
}

type environmentDeploymentsDataSource struct {
	client *CycleClient
}

type environmentDeploymentsDataSourceModel struct {
	ID            types.String                        `tfsdk:"id"`
	EnvironmentID types.String                        `tfsdk:"environment_id"`
	Versions      []environmentDeploymentVersionModel `tfsdk:"versions"`
	Tags          types.Map                           `tfsdk:"tags"`
}

type environmentDeploymentVersionModel struct {
	Version    types.String `tfsdk:"version"`
	Containers types.Int64  `tfsdk:"containers"`
	Tags       types.List   `tfsdk:"tags"`
}

func (d *environmentDeploymentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_deployments"
}

func (d *environmentDeploymentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the deployment versions in a Cycle environment (`GET /v1/environments/{id}/deployments`). " +
			"A version appears when containers set `deployment.version`. Environment tags (for example `prod`) point at a " +
			"version and are used by DNS LINKED records.\n\n" +
			"This is read-only. Tagging, starting, stopping, or pruning deployments is done via pipelines or Cycle jobs, " +
			"not this data source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The environment ID. Matches `environment_id`.",
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose deployments should be read.",
			},
			"versions": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Deployment versions present in the environment, sorted by version string.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"version": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The deployment version string (for example `v1.2.3`).",
						},
						"containers": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "The number of containers using this version.",
						},
						"tags": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Environment tags that currently point at this version.",
						},
					},
				},
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Map of environment tag to the version it currently points at (for example `prod` → `v1.2.3`).",
			},
		},
	}
}

func (d *environmentDeploymentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *environmentDeploymentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		return
	}

	var config environmentDeploymentsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	environmentID := config.EnvironmentID.ValueString()
	apiResp, err := d.client.Client.GetEnvironmentDeploymentsWithResponse(ctx, environmentID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading environment deployments", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Environment Not Found",
			fmt.Sprintf("No environment with ID %q exists in this hub.", environmentID),
		)
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading environment deployments", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(environmentDeploymentsFromAPI(ctx, &config, apiResp.JSON200.Data.Versions)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func environmentDeploymentsFromAPI(ctx context.Context, model *environmentDeploymentsDataSourceModel, versions map[string]struct {
	Containers int                `json:"containers"`
	Tags       []cycle.Identifier `json:"tags"`
}) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(model.EnvironmentID.ValueString())

	keys := make([]string, 0, len(versions))
	for version := range versions {
		keys = append(keys, version)
	}
	sort.Strings(keys)

	tagToVersion := make(map[string]string)
	model.Versions = make([]environmentDeploymentVersionModel, 0, len(keys))
	for _, version := range keys {
		info := versions[version]
		tags := make([]string, 0, len(info.Tags))
		for _, tag := range info.Tags {
			if tag == "" {
				continue
			}
			tags = append(tags, tag)
			tagToVersion[tag] = version
		}
		sort.Strings(tags)

		tagList, d := types.ListValueFrom(ctx, types.StringType, tags)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		model.Versions = append(model.Versions, environmentDeploymentVersionModel{
			Version:    types.StringValue(version),
			Containers: types.Int64Value(int64(info.Containers)),
			Tags:       tagList,
		})
	}

	tags, d := types.MapValueFrom(ctx, types.StringType, tagToVersion)
	diags.Append(d...)
	model.Tags = tags
	return diags
}
