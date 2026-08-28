package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewContainerResource)
}

var (
	_ resource.Resource                = (*containerResource)(nil)
	_ resource.ResourceWithConfigure   = (*containerResource)(nil)
	_ resource.ResourceWithImportState = (*containerResource)(nil)
)

// NewContainerResource returns the cycle_container resource.
func NewContainerResource() resource.Resource {
	return &containerResource{}
}

type containerResource struct {
	client *CycleClient
}

type containerResourceModel struct {
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
	StartOnCreate types.Bool           `tfsdk:"start_on_create"`
	HubID         types.String         `tfsdk:"hub_id"`
	State         types.String         `tfsdk:"state"`
	Instances     types.Int64          `tfsdk:"instances"`
	Deprecate     types.Bool           `tfsdk:"deprecate"`
}

func (r *containerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container"
}

func (r *containerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle container (`/v1/containers`). Containers package an application and " +
			"its dependencies and run as isolated processes in an environment.\n\n" +
			"The update API only accepts `name`, `identifier`, `lock`, `deprecate`, and `annotations`. " +
			"Changing `environment_id`, `image_id`, `stateful`, `config`, or `deployment` forces a new container.\n\n" +
			"`start_on_create` starts the container after create via a job. It is create-only and is not sent on update.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the container.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A user-defined name for the container.",
			},
			"identifier": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A human-readable slugged identifier for the container. Automatically generated from the name if not provided.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment this container is deployed into. Changing this forces a new container to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the image used to create this container. Changing this forces a new container to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"stateful": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "When `true`, this container is stateful. Changing this forces a new container to be created.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"config": schema.StringAttribute{
				Required:   true,
				CustomType: jsontypes.NormalizedType{},
				MarkdownDescription: "The Cycle container config object as normalized JSON. Required because the create API " +
					"models `config` as a non-pointer `Config` (at minimum `deploy` and `network`). Pass the object Cycle " +
					"expects, for example `jsonencode({ deploy = { instances = 1 }, network = { hostname = \"web\", public = \"disable\", egress_via_gateway = false } })`. " +
					"Changing this forces a new container to be created. Terraform keeps the configured JSON after apply; " +
					"the API often echoes additional defaulted fields that would otherwise force perpetual replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"deployment": schema.StringAttribute{
				Optional:   true,
				CustomType: jsontypes.NormalizedType{},
				MarkdownDescription: "Optional deployment descriptor as normalized JSON, for example `jsonencode({ version = \"1.0.0\" })`. " +
					"The update API does not accept this field; changing it forces a new container to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"annotations": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Custom metadata for the container. Not utilized by Cycle. Create sends `map[string]interface{}`; update sends `map[string]string`.",
			},
			"lock": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "When `true`, prevents this container from being deleted.",
			},
			"start_on_create": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, starts the container after create via a job. This is create-only and is not sent on update. Defaults to `false`.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this container belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the container (e.g. `new`, `running`, `stopped`).",
			},
			"instances": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The number of instances for this container.",
			},
			"deprecate": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the container is marked as deprecated.",
			},
		},
	}
}

func (r *containerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *containerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan containerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := containerConfigFromValue(plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deployment, diags := containerDeploymentFromValue(plan.Deployment)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	annotations, _, diags := containerAnnotationsFromValue(ctx, plan.Annotations)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.CreateContainerJSONRequestBody{
		Name:          plan.Name.ValueString(),
		EnvironmentId: plan.EnvironmentID.ValueString(),
		ImageId:       plan.ImageID.ValueString(),
		Stateful:      plan.Stateful.ValueBool(),
		Config:        *cfg,
		Deployment:    deployment,
		Annotations:   annotations,
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}
	if !plan.Lock.IsNull() && !plan.Lock.IsUnknown() {
		lock := plan.Lock.ValueBool()
		body.Lock = &lock
	}

	createResp, err := r.client.Client.CreateContainerWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating container", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating container", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(containerResourceModelFromAPI(ctx, &plan, createResp.JSON201.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.StartOnCreate.ValueBool() {
		if !startContainerJob(ctx, r.client, plan.ID.ValueString(), &resp.Diagnostics) {
			return
		}
		if !r.refreshContainer(ctx, &plan, &resp.Diagnostics) {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *containerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state containerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetContainerWithResponse(ctx, state.ID.ValueString(), &cycle.GetContainerParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading container", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading container", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	container := getResp.JSON200.Data
	if container.State.Current == cycle.ContainerStateCurrentDeleted || container.State.Current == cycle.ContainerStateCurrentDeleting {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(containerResourceModelFromAPI(ctx, &state, container)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.StartOnCreate.IsNull() || state.StartOnCreate.IsUnknown() {
		state.StartOnCreate = types.BoolValue(false)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *containerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan containerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state containerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	body := cycle.UpdateContainerJSONRequestBody{
		Name: &name,
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}
	if !plan.Lock.IsNull() && !plan.Lock.IsUnknown() {
		lock := plan.Lock.ValueBool()
		body.Lock = &lock
	}
	if !plan.Deprecate.IsNull() && !plan.Deprecate.IsUnknown() {
		deprecate := plan.Deprecate.ValueBool()
		body.Deprecate = &deprecate
	}
	_, annotations, diags := containerAnnotationsFromValue(ctx, plan.Annotations)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Annotation = annotations

	updateResp, err := r.client.Client.UpdateContainerWithResponse(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating container", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating container", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(containerResourceModelFromAPI(ctx, &plan, updateResp.JSON200.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *containerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state containerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteContainerWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting container", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting container", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}
	if job := deleteResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for container deletion", err.Error())
		}
	}
}

func (r *containerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *containerResource) refreshContainer(ctx context.Context, model *containerResourceModel, diags *diag.Diagnostics) bool {
	getResp, err := r.client.Client.GetContainerWithResponse(ctx, model.ID.ValueString(), &cycle.GetContainerParams{})
	if err != nil {
		diags.AddError("Error reading container", err.Error())
		return false
	}
	if getResp.JSON200 == nil {
		addAPIError(diags, "reading container", getResp.StatusCode(), getResp.JSONDefault)
		return false
	}
	diags.Append(containerResourceModelFromAPI(ctx, model, getResp.JSON200.Data)...)
	return !diags.HasError()
}

func startContainerJob(ctx context.Context, client *CycleClient, containerID string, diags *diag.Diagnostics) bool {
	var task cycle.ContainerTask
	if err := task.FromContainerStartAction(cycle.ContainerStartAction{
		Action: cycle.ContainerStartActionActionStart,
	}); err != nil {
		diags.AddError("Error starting container", err.Error())
		return false
	}

	jobResp, err := client.Client.CreateContainerJobWithResponse(ctx, containerID, task)
	if err != nil {
		diags.AddError("Error starting container", err.Error())
		return false
	}
	if jobResp.JSON202 == nil {
		addAPIError(diags, "starting container", jobResp.StatusCode(), jobResp.JSONDefault)
		return false
	}
	if job := jobResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, client, job.Id); err != nil {
			diags.AddError("Error waiting for container start", err.Error())
			return false
		}
	}
	return true
}

func containerResourceModelFromAPI(ctx context.Context, model *containerResourceModel, container cycle.Container) diag.Diagnostics {
	var diags diag.Diagnostics

	// config and deployment are create-only (RequiresReplace). Keep the
	// configured JSON when present so API-echoed defaults (e.g. network.routes)
	// do not force a replace on every plan.
	priorConfig := model.Config
	priorDeployment := model.Deployment

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
	}
	model.Stateful = types.BoolValue(container.Stateful)
	model.Lock = types.BoolValue(container.Lock)
	model.HubID = types.StringValue(container.HubId)
	model.State = types.StringValue(string(container.State.Current))
	model.Instances = types.Int64Value(int64(container.Instances))
	model.Deprecate = types.BoolValue(container.Deprecate)

	if priorConfig.IsNull() || priorConfig.IsUnknown() {
		cfg, d := containerConfigToValue(&container.Config)
		diags.Append(d...)
		model.Config = cfg
	} else {
		model.Config = priorConfig
	}

	if priorDeployment.IsNull() || priorDeployment.IsUnknown() {
		deployment, d := containerDeploymentToValue(container.Deployment)
		diags.Append(d...)
		model.Deployment = deployment
	} else {
		model.Deployment = priorDeployment
	}

	annotations, d := containerAnnotationsToValue(ctx, container.Annotations)
	diags.Append(d...)
	model.Annotations = annotations

	return diags
}

func containerConfigFromValue(value jsontypes.Normalized) (*cycle.Config, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		var diags diag.Diagnostics
		diags.AddError("Invalid container config", "config is required")
		return nil, diags
	}

	var cfg cycle.Config
	diags := value.Unmarshal(&cfg)
	if diags.HasError() {
		return nil, diags
	}
	return &cfg, nil
}

func containerConfigToValue(cfg *cycle.Config) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cfg == nil {
		return jsontypes.NewNormalizedNull(), diags
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		diags.AddError("Error encoding container config", err.Error())
		return jsontypes.NewNormalizedNull(), diags
	}
	if len(b) == 0 || string(b) == "null" {
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

func containerDeploymentFromValue(value jsontypes.Normalized) (*cycle.Deployment, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	raw := value.ValueString()
	if raw == "" || raw == "null" {
		return nil, nil
	}

	var deployment cycle.Deployment
	diags := value.Unmarshal(&deployment)
	if diags.HasError() {
		return nil, diags
	}
	return &deployment, nil
}

func containerDeploymentToValue(deployment *cycle.Deployment) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if deployment == nil {
		return jsontypes.NewNormalizedNull(), diags
	}

	b, err := json.Marshal(deployment)
	if err != nil {
		diags.AddError("Error encoding container deployment", err.Error())
		return jsontypes.NewNormalizedNull(), diags
	}
	if len(b) == 0 || string(b) == "null" {
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

func containerAnnotationsFromValue(ctx context.Context, m types.Map) (*map[string]interface{}, *map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m.IsNull() || m.IsUnknown() {
		return nil, nil, diags
	}

	strs := map[string]string{}
	diags.Append(m.ElementsAs(ctx, &strs, false)...)
	if diags.HasError() {
		return nil, nil, diags
	}

	iface := make(map[string]interface{}, len(strs))
	for k, v := range strs {
		iface[k] = v
	}
	return &iface, &strs, diags
}

func containerAnnotationsToValue(ctx context.Context, annotations *map[string]interface{}) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	if annotations == nil || len(*annotations) == 0 {
		return types.MapNull(types.StringType), diags
	}

	strs := make(map[string]string, len(*annotations))
	for k, v := range *annotations {
		switch val := v.(type) {
		case string:
			strs[k] = val
		case nil:
			strs[k] = ""
		default:
			b, err := json.Marshal(val)
			if err != nil {
				strs[k] = fmt.Sprint(val)
			} else {
				strs[k] = string(b)
			}
		}
	}

	m, d := types.MapValueFrom(ctx, types.StringType, strs)
	diags.Append(d...)
	return m, diags
}
