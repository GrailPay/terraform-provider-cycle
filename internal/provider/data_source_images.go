package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewImagesDataSource)
}

var (
	_ datasource.DataSource              = (*imagesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*imagesDataSource)(nil)
)

// NewImagesDataSource returns the cycle_images data source.
func NewImagesDataSource() datasource.DataSource {
	return &imagesDataSource{}
}

type imagesDataSource struct {
	client *CycleClient
}

type imagesDataSourceModel struct {
	SourceID types.String            `tfsdk:"source_id"`
	Images   []imageSummaryDataModel `tfsdk:"images"`
}

type imageSummaryDataModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	State types.String `tfsdk:"state"`
	Size  types.Int64  `tfsdk:"size"`
}

func (d *imagesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_images"
}

func (d *imagesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Cycle images in the hub, optionally filtered to those built from a specific " +
			"image source.",
		Attributes: map[string]schema.Attribute{
			"source_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "When set, only images created from this image source ID are returned.",
			},
			"images": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of images.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the image.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The user-defined name of the image.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the image.",
						},
						"size": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "The size of the image in bytes.",
						},
					},
				},
			},
		},
	}
}

func (d *imagesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *imagesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config imagesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.Images = []imageSummaryDataModel{}

	pageSize := float32(100)
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := pageSize
		params := &cycle.GetImagesParams{
			Page: &cycle.PageParam{Number: &number, Size: &size},
		}
		if !config.SourceID.IsNull() {
			sourceID := config.SourceID.ValueString()
			params.Filter = &struct {
				Identifier *string `json:"identifier,omitempty"`
				Search     *string `json:"search,omitempty"`
				SourceId   *string `json:"source_id,omitempty"`
				SourceType *string `json:"source_type,omitempty"`
				State      *string `json:"state,omitempty"`
			}{SourceId: &sourceID}
		}

		apiResp, err := d.client.Client.GetImagesWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("Error listing images", err.Error())
			return
		}
		if apiResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "listing images", apiResp.StatusCode(), apiResp.JSONDefault)
			return
		}

		for _, image := range apiResp.JSON200.Data {
			config.Images = append(config.Images, imageSummaryDataModel{
				ID:    types.StringValue(image.Id),
				Name:  types.StringValue(image.Name),
				State: types.StringValue(string(image.State.Current)),
				Size:  types.Int64Value(int64(image.Size)),
			})
		}

		if len(apiResp.JSON200.Data) < int(pageSize) {
			break
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
