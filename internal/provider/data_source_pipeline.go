package provider

import (
	"context"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewPipelineDataSource)
}

var (
	_ datasource.DataSource              = (*pipelineDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*pipelineDataSource)(nil)
)

// NewPipelineDataSource returns the cycle_pipeline data source.
func NewPipelineDataSource() datasource.DataSource {
	return &pipelineDataSource{}
}

type pipelineDataSource struct {
	client *CycleClient
}

type pipelineDataSourceModel struct {
	ID         types.String         `tfsdk:"id"`
	Name       types.String         `tfsdk:"name"`
	Identifier types.String         `tfsdk:"identifier"`
	Disable    types.Bool           `tfsdk:"disable"`
	Dynamic    types.Bool           `tfsdk:"dynamic"`
	Stages     jsontypes.Normalized `tfsdk:"stages"`
	HubID      types.String         `tfsdk:"hub_id"`
	State      types.String         `tfsdk:"state"`
}

func (d *pipelineDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline"
}

func (d *pipelineDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle pipeline by ID.",
		Attributes:          pipelineDataSourceAttributes(true),
	}
}

func (d *pipelineDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *pipelineDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config pipelineDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Client.GetPipelineWithResponse(ctx, config.ID.ValueString(), &cycle.GetPipelineParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading pipeline", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Pipeline Not Found",
			fmt.Sprintf("No pipeline with ID %q exists in this hub.", config.ID.ValueString()),
		)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading pipeline", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(pipelineDataSourceModelFromAPI(&config, getResp.JSON200.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func pipelineDataSourceAttributes(idRequired bool) map[string]schema.Attribute {
	id := schema.StringAttribute{
		MarkdownDescription: "The unique ID of the pipeline.",
	}
	if idRequired {
		id.Required = true
	} else {
		id.Computed = true
	}

	return map[string]schema.Attribute{
		"id": id,
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The user-defined name of the pipeline.",
		},
		"identifier": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The human-readable slugged identifier of the pipeline.",
		},
		"disable": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the pipeline is disabled and cannot be triggered.",
		},
		"dynamic": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether variable interpolation and other advanced logic is enabled on this pipeline.",
		},
		"stages": schema.StringAttribute{
			Computed:            true,
			CustomType:          jsontypes.NormalizedType{},
			MarkdownDescription: "The pipeline stages as normalized JSON. Null when the pipeline has no stages.",
		},
		"hub_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the hub this pipeline belongs to.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The current state of the pipeline.",
		},
	}
}

func pipelineDataSourceModelFromAPI(model *pipelineDataSourceModel, pipeline cycle.Pipeline) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ID = types.StringValue(pipeline.Id)
	model.Name = types.StringValue(pipeline.Name)
	if pipeline.Identifier != nil && *pipeline.Identifier != "" {
		model.Identifier = types.StringValue(*pipeline.Identifier)
	} else {
		model.Identifier = types.StringNull()
	}
	model.Disable = types.BoolValue(pipeline.Disable)
	model.Dynamic = types.BoolValue(pipeline.Dynamic)
	model.HubID = types.StringValue(pipeline.HubId)
	model.State = types.StringValue(string(pipeline.State.Current))

	stages, err := pipelineStagesFromAPI(pipeline.Stages)
	if err != nil {
		diags.AddError("Error encoding pipeline stages", err.Error())
		return diags
	}
	model.Stages = stages
	return diags
}
