package provider

import (
	"context"
	"fmt"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewImageSourceDataSource)
}

var (
	_ datasource.DataSource                     = (*imageSourceDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*imageSourceDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*imageSourceDataSource)(nil)
)

// NewImageSourceDataSource returns the cycle_image_source data source.
func NewImageSourceDataSource() datasource.DataSource {
	return &imageSourceDataSource{}
}

type imageSourceDataSource struct {
	client *CycleClient
}

type imageSourceDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Identifier  types.String `tfsdk:"identifier"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`
	State       types.String `tfsdk:"state"`
}

func (d *imageSourceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_image_source"
}

func (d *imageSourceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle image source by ID or by its human-readable identifier. Exactly one " +
			"of `id` or `identifier` must be set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique ID of the image source. Conflicts with `identifier`.",
			},
			"identifier": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "The human-readable slug identifier of the image source. Conflicts with `id`. " +
					"Identifiers are not guaranteed unique; the lookup fails if multiple image sources match.",
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
	}
}

func (d *imageSourceDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("identifier"),
		),
	}
}

func (d *imageSourceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *imageSourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config imageSourceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var source *cycle.ImageSource
	if !config.ID.IsNull() {
		apiResp, err := d.client.Client.GetImageSourceWithResponse(ctx, config.ID.ValueString(), &cycle.GetImageSourceParams{})
		if err != nil {
			resp.Diagnostics.AddError("Error reading image source", err.Error())
			return
		}
		if apiResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "reading image source", apiResp.StatusCode(), apiResp.JSONDefault)
			return
		}
		source = &apiResp.JSON200.Data
	} else {
		found, diags := d.findByIdentifier(ctx, config.Identifier.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		source = found
	}

	config.ID = types.StringValue(source.Id)
	config.Identifier = types.StringValue(source.Identifier)
	config.Name = types.StringValue(source.Name)
	config.Type = types.StringValue(string(source.Type))
	config.State = types.StringValue(string(source.State.Current))
	config.Description = types.StringNull()
	if source.About != nil && source.About.Description != nil {
		config.Description = types.StringValue(*source.About.Description)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// findByIdentifier resolves an image source via the identifier filter. The
// API allows duplicate identifiers, so exactly one match is required.
func (d *imageSourceDataSource) findByIdentifier(ctx context.Context, identifier string) (*cycle.ImageSource, diag.Diagnostics) {
	var diags diag.Diagnostics

	apiResp, err := d.client.Client.GetImageSourcesWithResponse(ctx, &cycle.GetImageSourcesParams{
		Filter: &struct {
			Identifier *string `json:"identifier,omitempty"`
			Search     *string `json:"search,omitempty"`
			State      *string `json:"state,omitempty"`
		}{Identifier: &identifier},
	})
	if err != nil {
		diags.AddError("Error listing image sources", err.Error())
		return nil, diags
	}
	if apiResp.JSON200 == nil {
		addAPIError(&diags, "listing image sources", apiResp.StatusCode(), apiResp.JSONDefault)
		return nil, diags
	}

	matches := apiResp.JSON200.Data
	switch len(matches) {
	case 0:
		diags.AddError(
			"Image Source Not Found",
			fmt.Sprintf("No image source with identifier %q exists in this hub.", identifier),
		)
		return nil, diags
	case 1:
		return &matches[0], diags
	default:
		diags.AddError(
			"Multiple Image Sources Found",
			fmt.Sprintf("%d image sources match identifier %q. Image source identifiers are not unique; use `id` instead.", len(matches), identifier),
		)
		return nil, diags
	}
}
