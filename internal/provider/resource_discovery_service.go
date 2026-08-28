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
	RegisterResource(NewDiscoveryServiceResource)
}

var (
	_ resource.Resource                = (*discoveryServiceResource)(nil)
	_ resource.ResourceWithConfigure   = (*discoveryServiceResource)(nil)
	_ resource.ResourceWithImportState = (*discoveryServiceResource)(nil)
)

// NewDiscoveryServiceResource returns the cycle_discovery_service resource.
func NewDiscoveryServiceResource() resource.Resource {
	return &discoveryServiceResource{}
}

type discoveryServiceResource struct {
	client *CycleClient
}

type discoveryServiceResourceModel struct {
	ID               types.String         `tfsdk:"id"`
	EnvironmentID    types.String         `tfsdk:"environment_id"`
	HighAvailability types.Bool           `tfsdk:"high_availability"`
	AutoUpdate       types.Bool           `tfsdk:"auto_update"`
	Config           jsontypes.Normalized `tfsdk:"config"`
	Enable           types.Bool           `tfsdk:"enable"`
	ContainerID      types.String         `tfsdk:"container_id"`
}

func (r *discoveryServiceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_discovery_service"
}

func (r *discoveryServiceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the discovery service for a Cycle environment. Discovery is an environment " +
			"singleton — it always exists and has no create or delete HTTP resource. Create and update send a " +
			"`reconfigure` job. There is no dedicated GET endpoint; Terraform reads `services.discovery` from " +
			"`GET /v1/environments/{id}`.\n\n" +
			"**Destroy is a state-only no-op.** Removing this resource from Terraform does not disable or " +
			"reconfigure the Cycle discovery service.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The environment ID this discovery service belongs to. The service is a singleton keyed on `environment_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose discovery service is managed. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"high_availability": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the discovery service runs in high availability mode.",
			},
			"auto_update": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the discovery service container is set to auto-update.",
			},
			"config": schema.StringAttribute{
				Optional:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Discovery configuration as a JSON object (`DiscoveryConfig`). Fields include `custom_resolvers`, `domain_suffix`, `dual_stack_legacy`, `empty_set_delay`, `external_resolution`, and `hosts`.",
			},
			"enable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the discovery service is currently enabled.",
			},
			"container_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the discovery service container, if one has been provisioned.",
			},
		},
	}
}

func (r *discoveryServiceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *discoveryServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan discoveryServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := discoveryConfigFromValue(plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureDiscoveryService(ctx, plan.EnvironmentID.ValueString(), servicesBoolPtr(plan.HighAvailability), servicesBoolPtr(plan.AutoUpdate), cfg, &resp.Diagnostics, "creating discovery service") {
		return
	}
	if !r.refreshDiscoveryService(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *discoveryServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state discoveryServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, status, envelope, err := getEnvironment(ctx, r.client, state.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading discovery service", err.Error())
		return
	}
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if env == nil {
		addAPIError(&resp.Diagnostics, "reading discovery service", status, envelope)
		return
	}

	resp.Diagnostics.Append(applyDiscoveryService(&state, env.Services.Discovery, discoveryManagesConfig(state.Config))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *discoveryServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		return
	}

	var plan discoveryServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := discoveryConfigFromValue(plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureDiscoveryService(ctx, plan.EnvironmentID.ValueString(), servicesBoolPtr(plan.HighAvailability), servicesBoolPtr(plan.AutoUpdate), cfg, &resp.Diagnostics, "updating discovery service") {
		return
	}
	if !r.refreshDiscoveryService(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *discoveryServiceResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	// State-only no-op: Cycle has no delete for the discovery service, and
	// destroy must not disable or reconfigure it.
}

func (r *discoveryServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *discoveryServiceResource) refreshDiscoveryService(ctx context.Context, model *discoveryServiceResourceModel, diags *diag.Diagnostics) bool {
	env, status, envelope, err := getEnvironment(ctx, r.client, model.EnvironmentID.ValueString())
	if err != nil {
		diags.AddError("Error reading discovery service", err.Error())
		return false
	}
	if env == nil {
		addAPIError(diags, "reading discovery service", status, envelope)
		return false
	}
	diags.Append(applyDiscoveryService(model, env.Services.Discovery, discoveryManagesConfig(model.Config))...)
	return !diags.HasError()
}

func (r *discoveryServiceResource) reconfigureDiscoveryService(ctx context.Context, environmentID string, highAvailability, autoUpdate *bool, config *cycle.DiscoveryConfig, diags *diag.Diagnostics, action string) bool {
	body := cycle.CreateDiscoveryServiceJobJSONRequestBody{
		Action: cycle.CreateDiscoveryServiceJobJSONBodyActionReconfigure,
	}
	body.Contents.HighAvailability = highAvailability
	body.Contents.AutoUpdate = autoUpdate
	body.Contents.Config = config

	apiResp, err := r.client.Client.CreateDiscoveryServiceJobWithResponse(ctx, environmentID, body)
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
			diags.AddError("Error waiting for discovery service reconfigure", err.Error())
			return false
		}
	}
	return true
}

func discoveryManagesConfig(config jsontypes.Normalized) bool {
	return !config.IsNull() && !config.IsUnknown()
}

func discoveryConfigFromValue(value jsontypes.Normalized) (*cycle.DiscoveryConfig, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var cfg cycle.DiscoveryConfig
	diags := value.Unmarshal(&cfg)
	if diags.HasError() {
		return nil, diags
	}
	return &cfg, nil
}

func discoveryConfigToValue(cfg *cycle.DiscoveryConfig) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cfg == nil {
		return jsontypes.NewNormalizedNull(), diags
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		diags.AddError("Error encoding discovery service config", err.Error())
		return jsontypes.NewNormalizedNull(), diags
	}
	if len(b) == 0 || string(b) == "null" {
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

func applyDiscoveryService(model *discoveryServiceResourceModel, svc *cycle.DiscoveryEnvironmentService, manageConfig bool) diag.Diagnostics {
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
	model.HighAvailability = types.BoolValue(svc.HighAvailability)
	if svc.AutoUpdate != nil {
		model.AutoUpdate = types.BoolValue(*svc.AutoUpdate)
	} else {
		model.AutoUpdate = types.BoolValue(false)
	}
	if svc.ContainerId != "" {
		model.ContainerID = types.StringValue(svc.ContainerId)
	} else {
		model.ContainerID = types.StringNull()
	}

	if manageConfig {
		cfg, d := discoveryConfigToValue(svc.Config)
		diags.Append(d...)
		model.Config = cfg
	}

	return diags
}

// getEnvironment loads an environment by ID. Used by the environment service
// resources, which have no dedicated GET endpoints.
func getEnvironment(ctx context.Context, client *CycleClient, environmentID string) (*cycle.Environment, int, *cycle.ErrorEnvelope, error) {
	apiResp, err := client.Client.GetEnvironmentWithResponse(ctx, environmentID, &cycle.GetEnvironmentParams{})
	if err != nil {
		return nil, 0, nil, err
	}
	if apiResp.JSON200 == nil {
		return nil, apiResp.StatusCode(), apiResp.JSONDefault, nil
	}
	return &apiResp.JSON200.Data, apiResp.StatusCode(), nil, nil
}
