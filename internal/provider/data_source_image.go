package provider

import (
	"context"
	"time"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewImageDataSource)
}

var (
	_ datasource.DataSource              = (*imageDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*imageDataSource)(nil)
)

// NewImageDataSource returns the cycle_image data source.
func NewImageDataSource() datasource.DataSource {
	return &imageDataSource{}
}

type imageDataSource struct {
	client *CycleClient
}

type imageDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	State       types.String `tfsdk:"state"`
	Size        types.Int64  `tfsdk:"size"`
	Description types.String `tfsdk:"description"`
	Created     types.String `tfsdk:"created"`
}

func (d *imageDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_image"
}

func (d *imageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle image (a point-in-time build of an image source) by ID. Images are " +
			"read-only artifacts created by the platform; use the `cycle_image_source` resource to control where " +
			"images come from.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique ID of the image.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The user-defined name of the image.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the image (e.g. `new`, `building`, `live`).",
			},
			"size": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The size of the image in bytes.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "A description of the image, if one has been set.",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The RFC 3339 timestamp of when the image was created.",
			},
		},
	}
}

func (d *imageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *imageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config imageDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.Client.GetImageWithResponse(ctx, config.ID.ValueString(), &cycle.GetImageParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading image", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading image", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	image := apiResp.JSON200.Data
	config.ID = types.StringValue(image.Id)
	config.Name = types.StringValue(image.Name)
	config.State = types.StringValue(string(image.State.Current))
	config.Size = types.Int64Value(int64(image.Size))
	config.Created = types.StringValue(image.Events.Created.Format(time.RFC3339))
	config.Description = types.StringNull()
	if image.About != nil && image.About.Description != nil {
		config.Description = types.StringValue(*image.About.Description)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
