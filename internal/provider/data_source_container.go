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
	RegisterDataSource(NewContainerDataSource)
}

var (
	_ datasource.DataSource              = (*containerDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*containerDataSource)(nil)
)

// NewContainerDataSource returns the cycle_container data source.
func NewContainerDataSource() datasource.DataSource {
	return &containerDataSource{}
}

type containerDataSource struct {
	client *CycleClient
}

type containerDataSourceModel struct {
	ID            types.String         `tfsdk:"id"`
	Name          types.String         `tfsdk:"name"`
	Identifier    types.String         `tfsdk:"identifier"`
	EnvironmentID types.String         `tfsdk:"environment_id"`
	ImageID       types.String         `tfsdk:"image_id"`
	Stateful      types.Bool           `tfsdk:"stateful"`
	Config        jsontypes.Normalized `tfsdk:"config"`
	Deployment    jsontypes.Normalized `tfsdk:"deployment"`
	Annotations   types.Map            `tfsdk:"annotations"`
	Lock          types.Bool           `tfsdk:"lock"`
	HubID         types.String         `tfsdk:"hub_id"`
	State         types.String         `tfsdk:"state"`
	Instances     types.Int64          `tfsdk:"instances"`
	Deprecate     types.Bool           `tfsdk:"deprecate"`
}

func (d *containerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container"
}

func (d *containerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle container by ID.",
		Attributes:          containerDataSourceAttributes(true),
	}
}

func (d *containerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *containerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config containerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Client.GetContainerWithResponse(ctx, config.ID.ValueString(), &cycle.GetContainerParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading container", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Container Not Found",
			fmt.Sprintf("No container with ID %q exists in this hub.", config.ID.ValueString()),
		)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading container", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(containerDataSourceModelFromAPI(ctx, &config, getResp.JSON200.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func containerDataSourceAttributes(idRequired bool) map[string]schema.Attribute {
	id := schema.StringAttribute{
		MarkdownDescription: "The unique ID of the container.",
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
			MarkdownDescription: "The user-defined name of the container.",
		},
		"identifier": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The human-readable slugged identifier of the container.",
		},
		"environment_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the environment this container is deployed into.",
		},
		"image_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the image used to create this container.",
		},
		"stateful": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether this container is stateful.",
		},
		"config": schema.StringAttribute{
			Computed:            true,
			CustomType:          jsontypes.NormalizedType{},
			MarkdownDescription: "The container configuration as normalized JSON.",
		},
		"deployment": schema.StringAttribute{
			Computed:            true,
			CustomType:          jsontypes.NormalizedType{},
			MarkdownDescription: "The deployment descriptor as normalized JSON, or null when unset.",
		},
		"annotations": schema.MapAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Custom metadata for the container.",
		},
		"lock": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether this container is locked against deletion.",
		},
		"hub_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the hub this container belongs to.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The current state of the container.",
		},
		"instances": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "The number of instances for this container.",
		},
		"deprecate": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the container is marked as deprecated.",
		},
	}
}

func containerDataSourceModelFromAPI(ctx context.Context, model *containerDataSourceModel, container cycle.Container) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(container.Id)
	model.Name = types.StringValue(container.Name)
	if container.Identifier != "" {
		model.Identifier = types.StringValue(container.Identifier)
	} else {
		model.Identifier = types.StringNull()
	}
	model.EnvironmentID = types.StringValue(container.Environment.Id)
	if container.Image.Id != nil && *container.Image.Id != "" {
		model.ImageID = types.StringValue(*container.Image.Id)
	} else {
		model.ImageID = types.StringNull()
	}
	model.Stateful = types.BoolValue(container.Stateful)
	model.Lock = types.BoolValue(container.Lock)
	model.HubID = types.StringValue(container.HubId)
	model.State = types.StringValue(string(container.State.Current))
	model.Instances = types.Int64Value(int64(container.Instances))
	model.Deprecate = types.BoolValue(container.Deprecate)

	cfg, d := containerConfigToValue(&container.Config)
	diags.Append(d...)
	model.Config = cfg

	deployment, d := containerDeploymentToValue(container.Deployment)
	diags.Append(d...)
	model.Deployment = deployment

	annotations, d := containerAnnotationsToValue(ctx, container.Annotations)
	diags.Append(d...)
	model.Annotations = annotations

	return diags
}
