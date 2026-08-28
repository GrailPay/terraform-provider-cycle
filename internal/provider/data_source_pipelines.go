package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func init() {
	RegisterDataSource(NewPipelinesDataSource)
}

var (
	_ datasource.DataSource              = (*pipelinesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*pipelinesDataSource)(nil)
)

// NewPipelinesDataSource returns the cycle_pipelines data source.
func NewPipelinesDataSource() datasource.DataSource {
	return &pipelinesDataSource{}
}

type pipelinesDataSource struct {
	client *CycleClient
}

type pipelinesDataSourceModel struct {
	Pipelines []pipelineDataSourceModel `tfsdk:"pipelines"`
}

func (d *pipelinesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipelines"
}

func (d *pipelinesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Cycle pipelines in the hub.",
		Attributes: map[string]schema.Attribute{
			"pipelines": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All pipelines in the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: pipelineDataSourceAttributes(false),
				},
			},
		},
	}
}

func (d *pipelinesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *pipelinesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config pipelinesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pipelines, err := fetchAllPipelines(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing pipelines", err.Error())
		return
	}

	config.Pipelines = make([]pipelineDataSourceModel, 0, len(pipelines))
	for _, pipeline := range pipelines {
		var item pipelineDataSourceModel
		resp.Diagnostics.Append(pipelineDataSourceModelFromAPI(&item, pipeline)...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.Pipelines = append(config.Pipelines, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// fetchAllPipelines pages through GET /v1/pipelines and returns every pipeline
// in the hub.
func fetchAllPipelines(ctx context.Context, client *CycleClient) ([]cycle.Pipeline, error) {
	const pageSize = 100

	var all []cycle.Pipeline
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetPipelinesWithResponse(ctx, &cycle.GetPipelinesParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing pipelines", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
