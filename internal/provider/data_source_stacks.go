package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func init() {
	RegisterDataSource(NewStacksDataSource)
}

var (
	_ datasource.DataSource              = (*stacksDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*stacksDataSource)(nil)
)

// NewStacksDataSource returns the cycle_stacks data source.
func NewStacksDataSource() datasource.DataSource {
	return &stacksDataSource{}
}

type stacksDataSource struct {
	client *CycleClient
}

type stacksDataSourceModel struct {
	Stacks []stackDataSourceModel `tfsdk:"stacks"`
}

func (d *stacksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stacks"
}

func (d *stacksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Cycle stacks in the hub.",
		Attributes: map[string]schema.Attribute{
			"stacks": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All stacks in the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: stackDataSourceAttributes(false),
				},
			},
		},
	}
}

func (d *stacksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *stacksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config stacksDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stacks, err := fetchAllStacks(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing stacks", err.Error())
		return
	}

	config.Stacks = make([]stackDataSourceModel, 0, len(stacks))
	for _, stack := range stacks {
		var item stackDataSourceModel
		resp.Diagnostics.Append(stackDataSourceModelFromAPI(ctx, &item, stack)...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.Stacks = append(config.Stacks, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// fetchAllStacks pages through GET /v1/stacks and returns every stack in the hub.
func fetchAllStacks(ctx context.Context, client *CycleClient) ([]cycle.Stack, error) {
	const pageSize = 100

	var all []cycle.Stack
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetStacksWithResponse(ctx, &cycle.GetStacksParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing stacks", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
