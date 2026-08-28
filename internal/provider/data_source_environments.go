package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewEnvironmentsDataSource)
}

var (
	_ datasource.DataSource              = (*environmentsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*environmentsDataSource)(nil)
)

// NewEnvironmentsDataSource returns the cycle_environments data source.
func NewEnvironmentsDataSource() datasource.DataSource {
	return &environmentsDataSource{}
}

type environmentsDataSource struct {
	client *CycleClient
}

type environmentsDataSourceModel struct {
	Environments []environmentsDataSourceItemModel `tfsdk:"environments"`
}

type environmentsDataSourceItemModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Identifier       types.String `tfsdk:"identifier"`
	Cluster          types.String `tfsdk:"cluster"`
	Description      types.String `tfsdk:"description"`
	LegacyNetworking types.Bool   `tfsdk:"legacy_networking"`
	HubID            types.String `tfsdk:"hub_id"`
	State            types.String `tfsdk:"state"`
}

func (d *environmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environments"
}

func (d *environmentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Cycle environments in the hub.",
		Attributes: map[string]schema.Attribute{
			"environments": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All environments in the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the environment.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The user-defined name of the environment.",
						},
						"identifier": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The human-readable slugged identifier of the environment.",
						},
						"cluster": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The identifier of the cluster this environment is deployed into.",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The custom description of the environment.",
						},
						"legacy_networking": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether legacy networking mode is enabled on this environment.",
						},
						"hub_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the hub this environment belongs to.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the environment.",
						},
					},
				},
			},
		},
	}
}

func (d *environmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *environmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config environmentsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	environments, err := fetchAllEnvironments(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing environments", err.Error())
		return
	}

	config.Environments = make([]environmentsDataSourceItemModel, 0, len(environments))
	for _, env := range environments {
		config.Environments = append(config.Environments, environmentsDataSourceItemModel{
			ID:               types.StringValue(env.Id),
			Name:             types.StringValue(env.Name),
			Identifier:       types.StringValue(env.Identifier),
			Cluster:          types.StringValue(env.Cluster),
			Description:      types.StringValue(env.About.Description),
			LegacyNetworking: types.BoolValue(env.Features.LegacyNetworking),
			HubID:            types.StringValue(env.HubId),
			State:            types.StringValue(string(env.State.Current)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// fetchAllEnvironments pages through GET /v1/environments and returns every
// environment in the hub. Shared by the cycle_environment and
// cycle_environments data sources.
func fetchAllEnvironments(ctx context.Context, client *CycleClient) ([]cycle.Environment, error) {
	const pageSize = 100

	var all []cycle.Environment
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetEnvironmentsWithResponse(ctx, &cycle.GetEnvironmentsParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing environments", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
