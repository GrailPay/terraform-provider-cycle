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
	RegisterResource(NewLoadBalancerResource)
}

var (
	_ resource.Resource                = (*loadBalancerResource)(nil)
	_ resource.ResourceWithConfigure   = (*loadBalancerResource)(nil)
	_ resource.ResourceWithImportState = (*loadBalancerResource)(nil)
)

// NewLoadBalancerResource returns the cycle_load_balancer resource.
func NewLoadBalancerResource() resource.Resource {
	return &loadBalancerResource{}
}

type loadBalancerResource struct {
	client *CycleClient
}

type loadBalancerResourceModel struct {
	ID               types.String         `tfsdk:"id"`
	EnvironmentID    types.String         `tfsdk:"environment_id"`
	HighAvailability types.Bool           `tfsdk:"high_availability"`
	AutoUpdate       types.Bool           `tfsdk:"auto_update"`
	Config           jsontypes.Normalized `tfsdk:"config"`
	Enable           types.Bool           `tfsdk:"enable"`
	ContainerID      types.String         `tfsdk:"container_id"`
	CurrentType      types.String         `tfsdk:"current_type"`
}

func (r *loadBalancerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_load_balancer"
}

func (r *loadBalancerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the load balancer service for a Cycle environment. The load balancer is an environment singleton — it always exists and has no create or delete HTTP resource. Create and update send a `reconfigure` job; destroy resets high availability and auto-update and removes the resource from state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The environment ID this load balancer belongs to. The load balancer is a singleton keyed on `environment_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose load balancer is managed. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"high_availability": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the load balancer service runs in high availability mode.",
			},
			"auto_update": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the load balancer service container is set to auto-update.",
			},
			"config": schema.StringAttribute{
				Optional:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Load balancer configuration as a JSON object. The Cycle API models this as a discriminated union (`default`, `haproxy`, or `v1`). Pass the object Cycle expects, for example `jsonencode({ type = \"haproxy\", ipv4 = true, ipv6 = false, performance = false, details = { ... } })`.",
			},
			"enable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the load balancer service is currently enabled.",
			},
			"container_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the load balancer service container, if one has been provisioned.",
			},
			"current_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The load balancer implementation currently in use (`haproxy` or `v1`).",
			},
		},
	}
}

func (r *loadBalancerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *loadBalancerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan loadBalancerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := loadBalancerConfigFromValue(plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureLoadBalancer(ctx, plan.EnvironmentID.ValueString(), servicesBoolPtr(plan.HighAvailability), servicesBoolPtr(plan.AutoUpdate), cfg, &resp.Diagnostics, "creating load balancer") {
		return
	}

	if !r.refreshLoadBalancer(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *loadBalancerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state loadBalancerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	info, status, envelope, err := r.getLoadBalancer(ctx, state.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading load balancer", err.Error())
		return
	}
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if info == nil {
		addAPIError(&resp.Diagnostics, "reading load balancer", status, envelope)
		return
	}

	resp.Diagnostics.Append(applyLoadBalancerInfo(&state, *info, loadBalancerManagesConfig(state.Config))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *loadBalancerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		return
	}

	var plan loadBalancerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := loadBalancerConfigFromValue(plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureLoadBalancer(ctx, plan.EnvironmentID.ValueString(), servicesBoolPtr(plan.HighAvailability), servicesBoolPtr(plan.AutoUpdate), cfg, &resp.Diagnostics, "updating load balancer") {
		return
	}

	if !r.refreshLoadBalancer(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *loadBalancerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}

	var state loadBalancerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The LB service cannot be deleted. Reset HA / auto-update and drop config,
	// then remove the resource from state.
	ha := false
	autoUpdate := false
	if !r.reconfigureLoadBalancer(ctx, state.EnvironmentID.ValueString(), &ha, &autoUpdate, nil, &resp.Diagnostics, "resetting load balancer") {
		// A missing environment means there is nothing left to reset.
		if resp.Diagnostics.HasError() {
			return
		}
	}
}

func (r *loadBalancerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *loadBalancerResource) getLoadBalancer(ctx context.Context, environmentID string) (*cycle.LoadBalancerInfo, int, *cycle.ErrorEnvelope, error) {
	apiResp, err := r.client.Client.GetLoadBalancerServiceWithResponse(ctx, environmentID)
	if err != nil {
		return nil, 0, nil, err
	}
	if apiResp.JSON200 == nil {
		return nil, apiResp.StatusCode(), apiResp.JSONDefault, nil
	}
	return &apiResp.JSON200.Data, apiResp.StatusCode(), nil, nil
}

func (r *loadBalancerResource) refreshLoadBalancer(ctx context.Context, model *loadBalancerResourceModel, diags *diag.Diagnostics) bool {
	info, status, envelope, err := r.getLoadBalancer(ctx, model.EnvironmentID.ValueString())
	if err != nil {
		diags.AddError("Error reading load balancer", err.Error())
		return false
	}
	if info == nil {
		addAPIError(diags, "reading load balancer", status, envelope)
		return false
	}
	diags.Append(applyLoadBalancerInfo(model, *info, loadBalancerManagesConfig(model.Config))...)
	return !diags.HasError()
}

func (r *loadBalancerResource) reconfigureLoadBalancer(ctx context.Context, environmentID string, highAvailability, autoUpdate *bool, config *cycle.LoadBalancerConfig, diags *diag.Diagnostics, action string) bool {
	body := cycle.CreateLoadBalancerServiceJobJSONRequestBody{
		Action: cycle.CreateLoadBalancerServiceJobJSONBodyActionReconfigure,
	}
	body.Contents.HighAvailability = highAvailability
	body.Contents.AutoUpdate = autoUpdate
	body.Contents.Config = config

	apiResp, err := r.client.Client.CreateLoadBalancerServiceJobWithResponse(ctx, environmentID, body)
	if err != nil {
		diags.AddError("Error "+action, err.Error())
		return false
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return true
	}
	if apiResp.JSON202 == nil {
		addAPIError(diags, action, apiResp.StatusCode(), apiResp.JSONDefault)
		return false
	}
	if job := apiResp.JSON202.Data.Job; job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
			diags.AddError("Error waiting for load balancer reconfigure", err.Error())
			return false
		}
	}
	return true
}

func loadBalancerManagesConfig(config jsontypes.Normalized) bool {
	return !config.IsNull() && !config.IsUnknown()
}

func loadBalancerConfigFromValue(value jsontypes.Normalized) (*cycle.LoadBalancerConfig, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var cfg cycle.LoadBalancerConfig
	diags := value.Unmarshal(&cfg)
	if diags.HasError() {
		return nil, diags
	}
	return &cfg, nil
}

func loadBalancerConfigToValue(cfg *cycle.LoadBalancerConfig) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cfg == nil {
		return jsontypes.NewNormalizedNull(), diags
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		diags.AddError("Error encoding load balancer config", err.Error())
		return jsontypes.NewNormalizedNull(), diags
	}
	if len(b) == 0 || string(b) == "null" {
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

func applyLoadBalancerInfo(model *loadBalancerResourceModel, info cycle.LoadBalancerInfo, manageConfig bool) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(model.EnvironmentID.ValueString())
	model.CurrentType = types.StringValue(string(info.CurrentType))

	if info.Service == nil {
		model.Enable = types.BoolValue(false)
		model.HighAvailability = types.BoolValue(false)
		model.AutoUpdate = types.BoolValue(false)
		model.ContainerID = types.StringNull()
		if manageConfig {
			model.Config = jsontypes.NewNormalizedNull()
		}
		return diags
	}

	svc := info.Service
	model.Enable = types.BoolValue(svc.Enable)
	model.HighAvailability = types.BoolValue(svc.HighAvailability)
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
		cfg, d := loadBalancerConfigToValue(svc.Config)
		diags.Append(d...)
		model.Config = cfg
	}

	return diags
}

func servicesBoolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}
