package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewProviderServerModelsDataSource)
}

var (
	_ datasource.DataSource              = (*providerServerModelsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*providerServerModelsDataSource)(nil)
)

// NewProviderServerModelsDataSource returns the cycle_provider_server_models data source.
func NewProviderServerModelsDataSource() datasource.DataSource {
	return &providerServerModelsDataSource{}
}

type providerServerModelsDataSource struct {
	client *CycleClient
}

type providerServerModelsDataSourceModel struct {
	IntegrationID types.String                              `tfsdk:"integration_id"`
	Models        []providerServerModelsDataSourceItemModel `tfsdk:"models"`
}

type providerServerModelsDataSourceItemModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Compatible  types.Bool   `tfsdk:"compatible"`
	LowResource types.Bool   `tfsdk:"low_resource"`
	LocationIDs types.List   `tfsdk:"location_ids"`
	Model       types.String `tfsdk:"model"`
	Category    types.String `tfsdk:"category"`
}

func (d *providerServerModelsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider_server_models"
}

func (d *providerServerModelsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists server models available from a Cycle provider integration. Use this to choose a `model_id` for `cycle_server`.",
		Attributes: map[string]schema.Attribute{
			"integration_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The provider integration ID to list server models for.",
			},
			"models": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Server models advertised by the provider integration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The server model ID.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The server model name.",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "A description of the server model.",
						},
						"compatible": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether Cycle currently supports this model.",
						},
						"low_resource": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this model has limited resources and should only be used for lightweight workloads.",
						},
						"location_ids": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Location IDs where this model is available.",
						},
						"model": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The provider-specific model identifier.",
						},
						"category": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The provider category for this model.",
						},
					},
				},
			},
		},
	}
}

func (d *providerServerModelsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *providerServerModelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config providerServerModelsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	models, err := fetchAllProviderServerModels(ctx, d.client, config.IntegrationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing provider server models", err.Error())
		return
	}

	config.Models = make([]providerServerModelsDataSourceItemModel, 0, len(models))
	for _, model := range models {
		locationIDs := model.LocationIds
		if locationIDs == nil {
			locationIDs = []string{}
		}
		ids, diags := types.ListValueFrom(ctx, types.StringType, locationIDs)
		resp.Diagnostics.Append(diags...)

		config.Models = append(config.Models, providerServerModelsDataSourceItemModel{
			ID:          types.StringValue(model.Id),
			Name:        types.StringValue(model.Name),
			Description: types.StringValue(model.Description),
			Compatible:  types.BoolValue(model.Compatible),
			LowResource: types.BoolValue(model.LowResource),
			LocationIDs: ids,
			Model:       types.StringValue(model.Provider.Model),
			Category:    types.StringValue(model.Provider.Category),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func fetchAllProviderServerModels(ctx context.Context, client *CycleClient, integrationID string) ([]cycle.ProviderServerModel, error) {
	const pageSize = 100

	var all []cycle.ProviderServerModel
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetProviderServerModelsWithResponse(ctx, integrationID, &cycle.GetProviderServerModelsParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing provider server models", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
