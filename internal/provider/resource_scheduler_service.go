package provider

import (
	"context"
	"encoding/json"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewSchedulerServiceResource)
}

var (
	_ resource.Resource                = (*schedulerServiceResource)(nil)
	_ resource.ResourceWithConfigure   = (*schedulerServiceResource)(nil)
	_ resource.ResourceWithImportState = (*schedulerServiceResource)(nil)
)

// NewSchedulerServiceResource returns the cycle_scheduler_service resource.
func NewSchedulerServiceResource() resource.Resource {
	return &schedulerServiceResource{}
}

type schedulerServiceResource struct {
	client *CycleClient
}

type schedulerServiceResourceModel struct {
	ID               types.String         `tfsdk:"id"`
	EnvironmentID    types.String         `tfsdk:"environment_id"`
	AutoUpdate       types.Bool           `tfsdk:"auto_update"`
	Config           jsontypes.Normalized `tfsdk:"config"`
	Enable           types.Bool           `tfsdk:"enable"`
	ContainerID      types.String         `tfsdk:"container_id"`
	HighAvailability types.Bool           `tfsdk:"high_availability"`
}

func (r *schedulerServiceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scheduler_service"
}

func (r *schedulerServiceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the scheduler service for a Cycle environment. The scheduler is an environment " +
			"singleton used by function-strategy containers. Create and update send a `reconfigure` job. There is " +
			"no dedicated GET endpoint; Terraform reads `services.scheduler` from `GET /v1/environments/{id}`.\n\n" +
			"High availability is not part of the reconfigure job body and is always `false` as of the Cycle API. " +
			"**Destroy is a state-only no-op.** Removing this resource from Terraform does not disable or " +
			"reconfigure the Cycle scheduler service.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The environment ID this scheduler service belongs to. The service is a singleton keyed on `environment_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose scheduler service is managed. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"auto_update": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the scheduler service container is set to auto-update.",
			},
			"config": schema.StringAttribute{
				Optional:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Scheduler configuration as a JSON object (`SchedulerConfig`). Fields: `public` (bool) and optional `access_keys`.",
			},
			"enable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the scheduler service is currently enabled.",
			},
			"container_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the scheduler service container, if one has been provisioned.",
			},
			"high_availability": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the scheduler service is running in high availability mode. The Cycle API reports this as always `false`.",
			},
		},
	}
}

func (r *schedulerServiceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *schedulerServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan schedulerServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := schedulerConfigFromValue(plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureSchedulerService(ctx, plan.EnvironmentID.ValueString(), servicesBoolPtr(plan.AutoUpdate), cfg, &resp.Diagnostics, "creating scheduler service") {
		return
	}
	if !r.refreshSchedulerService(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *schedulerServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state schedulerServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, status, envelope, err := getEnvironment(ctx, r.client, state.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading scheduler service", err.Error())
		return
	}
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if env == nil {
		addAPIError(&resp.Diagnostics, "reading scheduler service", status, envelope)
		return
	}

	resp.Diagnostics.Append(applySchedulerService(&state, env.Services.Scheduler, schedulerManagesConfig(state.Config))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *schedulerServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		return
	}

	var plan schedulerServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := schedulerConfigFromValue(plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureSchedulerService(ctx, plan.EnvironmentID.ValueString(), servicesBoolPtr(plan.AutoUpdate), cfg, &resp.Diagnostics, "updating scheduler service") {
		return
	}
	if !r.refreshSchedulerService(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *schedulerServiceResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	// State-only no-op: Cycle has no delete for the scheduler service, and
	// destroy must not disable or reconfigure it.
}

func (r *schedulerServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *schedulerServiceResource) refreshSchedulerService(ctx context.Context, model *schedulerServiceResourceModel, diags *diag.Diagnostics) bool {
	env, status, envelope, err := getEnvironment(ctx, r.client, model.EnvironmentID.ValueString())
	if err != nil {
		diags.AddError("Error reading scheduler service", err.Error())
		return false
	}
	if env == nil {
		addAPIError(diags, "reading scheduler service", status, envelope)
		return false
	}
	diags.Append(applySchedulerService(model, env.Services.Scheduler, schedulerManagesConfig(model.Config))...)
	return !diags.HasError()
}

func (r *schedulerServiceResource) reconfigureSchedulerService(ctx context.Context, environmentID string, autoUpdate *bool, config *cycle.SchedulerConfig, diags *diag.Diagnostics, action string) bool {
	body := cycle.CreateSchedulerServiceJobJSONRequestBody{
		Action: cycle.CreateSchedulerServiceJobJSONBodyActionReconfigure,
	}
	body.Contents.AutoUpdate = autoUpdate
	body.Contents.Config = config

	apiResp, err := r.client.Client.CreateSchedulerServiceJobWithResponse(ctx, environmentID, body)
	if err != nil {
		diags.AddError("Error "+action, err.Error())
		return false
	}
	if apiResp.JSON202 == nil {
		addAPIError(diags, action, apiResp.StatusCode(), apiResp.JSONDefault)
		return false
	}
	if job := apiResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, r.client, job.Id); err != nil {
			diags.AddError("Error waiting for scheduler service reconfigure", err.Error())
			return false
		}
	}
	return true
}

func schedulerManagesConfig(config jsontypes.Normalized) bool {
	return !config.IsNull() && !config.IsUnknown()
}

func schedulerConfigFromValue(value jsontypes.Normalized) (*cycle.SchedulerConfig, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var cfg cycle.SchedulerConfig
	diags := value.Unmarshal(&cfg)
	if diags.HasError() {
		return nil, diags
	}
	return &cfg, nil
}

func schedulerConfigToValue(cfg *cycle.SchedulerConfig) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cfg == nil {
		return jsontypes.NewNormalizedNull(), diags
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		diags.AddError("Error encoding scheduler service config", err.Error())
		return jsontypes.NewNormalizedNull(), diags
	}
	if len(b) == 0 || string(b) == "null" {
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

func applySchedulerService(model *schedulerServiceResourceModel, svc *cycle.SchedulerEnvironmentService, manageConfig bool) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(model.EnvironmentID.ValueString())

	if svc == nil {
		model.Enable = types.BoolValue(false)
		model.HighAvailability = types.BoolValue(false)
		model.AutoUpdate = types.BoolValue(false)
		model.ContainerID = types.StringNull()
		if manageConfig {
			model.Config = jsontypes.NewNormalizedNull()
		}
		return diags
	}

	model.Enable = types.BoolValue(svc.Enable)
	if svc.HighAvailability != nil {
		model.HighAvailability = types.BoolValue(*svc.HighAvailability)
	} else {
		model.HighAvailability = types.BoolValue(false)
	}
	if svc.AutoUpdate != nil {
		model.AutoUpdate = types.BoolValue(*svc.AutoUpdate)
	} else {
		model.AutoUpdate = types.BoolValue(false)
	}
	if svc.ContainerId != nil && *svc.ContainerId != "" {
		model.ContainerID = types.StringValue(*svc.ContainerId)
	} else {
		model.ContainerID = types.StringNull()
	}

	if manageConfig {
		cfg, d := schedulerConfigToValue(svc.Config)
		diags.Append(d...)
		model.Config = cfg
	}

	return diags
}
