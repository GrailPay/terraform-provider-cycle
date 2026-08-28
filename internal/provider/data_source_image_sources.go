package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewImageSourcesDataSource)
}

var (
	_ datasource.DataSource              = (*imageSourcesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*imageSourcesDataSource)(nil)
)

// NewImageSourcesDataSource returns the cycle_image_sources data source.
func NewImageSourcesDataSource() datasource.DataSource {
	return &imageSourcesDataSource{}
}

type imageSourcesDataSource struct {
	client *CycleClient
}

type imageSourcesDataSourceModel struct {
	Sources []imageSourceDataSourceModel `tfsdk:"sources"`
}

func (d *imageSourcesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_image_sources"
}

func (d *imageSourcesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Cycle image sources in the hub.",
		Attributes: map[string]schema.Attribute{
			"sources": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The image sources in the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the image source.",
						},
						"identifier": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The human-readable slug identifier of the image source.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the image source.",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The type of images in this source (`direct`, `stack-build`, or `bucket`).",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The description of the image source, if one has been set.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the image source.",
						},
					},
				},
			},
		},
	}
}

func (d *imageSourcesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *imageSourcesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config imageSourcesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sources, err := fetchAllImageSources(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing image sources", err.Error())
		return
	}

	config.Sources = make([]imageSourceDataSourceModel, 0, len(sources))
	for _, source := range sources {
		item := imageSourceDataSourceModel{
			ID:          types.StringValue(source.Id),
			Identifier:  types.StringValue(source.Identifier),
			Name:        types.StringValue(source.Name),
			Type:        types.StringValue(string(source.Type)),
			State:       types.StringValue(string(source.State.Current)),
			Description: types.StringNull(),
		}
		if source.About != nil && source.About.Description != nil {
			item.Description = types.StringValue(*source.About.Description)
		}
		config.Sources = append(config.Sources, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func fetchAllImageSources(ctx context.Context, client *CycleClient) ([]cycle.ImageSource, error) {
	const pageSize = 100

	var all []cycle.ImageSource
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		apiResp, err := client.Client.GetImageSourcesWithResponse(ctx, &cycle.GetImageSourcesParams{
			Page: &cycle.PageParam{Number: &number, Size: &size},
		})
		if err != nil {
			return nil, err
		}
		if apiResp.JSON200 == nil {
			return nil, apiError("listing image sources", apiResp.StatusCode(), apiResp.JSONDefault)
		}

		all = append(all, apiResp.JSON200.Data...)
		if len(apiResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
