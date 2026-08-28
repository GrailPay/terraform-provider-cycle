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
	RegisterResource(NewGatewayServiceResource)
}

var (
	_ resource.Resource                = (*gatewayServiceResource)(nil)
	_ resource.ResourceWithConfigure   = (*gatewayServiceResource)(nil)
	_ resource.ResourceWithImportState = (*gatewayServiceResource)(nil)
)

// NewGatewayServiceResource returns the cycle_gateway_service resource.
func NewGatewayServiceResource() resource.Resource {
	return &gatewayServiceResource{}
}

type gatewayServiceResource struct {
	client *CycleClient
}

type gatewayServiceResourceModel struct {
	ID               types.String         `tfsdk:"id"`
	EnvironmentID    types.String         `tfsdk:"environment_id"`
	AutoUpdate       types.Bool           `tfsdk:"auto_update"`
	Config           jsontypes.Normalized `tfsdk:"config"`
	Enable           types.Bool           `tfsdk:"enable"`
	ContainerID      types.String         `tfsdk:"container_id"`
	HighAvailability types.Bool           `tfsdk:"high_availability"`
}

func (r *gatewayServiceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gateway_service"
}

func (r *gatewayServiceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the gateway service for a Cycle environment. The gateway is an environment " +
			"singleton — it always exists and has no create or delete HTTP resource. Create and update send a " +
			"`reconfigure` job. There is no dedicated GET endpoint; Terraform reads `services.gateway` from " +
			"`GET /v1/environments/{id}`.\n\n" +
			"High availability is not part of the reconfigure job body; it is computed from the environment. " +
			"**Destroy is a state-only no-op.** Removing this resource from Terraform does not disable or " +
			"reconfigure the Cycle gateway service.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The environment ID this gateway service belongs to. The service is a singleton keyed on `environment_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose gateway service is managed. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"auto_update": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the gateway service container is set to auto-update.",
			},
			"config": schema.StringAttribute{
				Optional:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Gateway configuration as a JSON object (`GatewayConfig`). Fields: `ipv4`, `ipv6`, and `performance` (all booleans).",
			},
			"enable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the gateway service is currently enabled.",
			},
			"container_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the gateway service container, if one has been provisioned.",
			},
			"high_availability": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the gateway service is running in high availability mode. This value is read-only; the reconfigure job does not accept it.",
			},
		},
	}
}

func (r *gatewayServiceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *gatewayServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan gatewayServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := gatewayConfigFromValue(plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureGatewayService(ctx, plan.EnvironmentID.ValueString(), servicesBoolPtr(plan.AutoUpdate), cfg, &resp.Diagnostics, "creating gateway service") {
		return
	}
	if !r.refreshGatewayService(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gatewayServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state gatewayServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, status, envelope, err := getEnvironment(ctx, r.client, state.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading gateway service", err.Error())
		return
	}
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if env == nil {
		addAPIError(&resp.Diagnostics, "reading gateway service", status, envelope)
		return
	}

	resp.Diagnostics.Append(applyGatewayService(&state, env.Services.Gateway, gatewayManagesConfig(state.Config))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gatewayServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		return
	}

	var plan gatewayServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := gatewayConfigFromValue(plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureGatewayService(ctx, plan.EnvironmentID.ValueString(), servicesBoolPtr(plan.AutoUpdate), cfg, &resp.Diagnostics, "updating gateway service") {
		return
	}
	if !r.refreshGatewayService(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gatewayServiceResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	// State-only no-op: Cycle has no delete for the gateway service, and
	// destroy must not disable or reconfigure it.
}

func (r *gatewayServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *gatewayServiceResource) refreshGatewayService(ctx context.Context, model *gatewayServiceResourceModel, diags *diag.Diagnostics) bool {
	env, status, envelope, err := getEnvironment(ctx, r.client, model.EnvironmentID.ValueString())
	if err != nil {
		diags.AddError("Error reading gateway service", err.Error())
		return false
	}
	if env == nil {
		addAPIError(diags, "reading gateway service", status, envelope)
		return false
	}
	diags.Append(applyGatewayService(model, env.Services.Gateway, gatewayManagesConfig(model.Config))...)
	return !diags.HasError()
}

func (r *gatewayServiceResource) reconfigureGatewayService(ctx context.Context, environmentID string, autoUpdate *bool, config *cycle.GatewayConfig, diags *diag.Diagnostics, action string) bool {
	body := cycle.CreateGatewayServiceJobJSONRequestBody{
		Action: cycle.CreateGatewayServiceJobJSONBodyActionReconfigure,
	}
	body.Contents.AutoUpdate = autoUpdate
	body.Contents.Config = config

	apiResp, err := r.client.Client.CreateGatewayServiceJobWithResponse(ctx, environmentID, body)
	if err != nil {
		diags.AddError("Error "+action, err.Error())
		return false
	}
	if apiResp.JSON202 == nil {
		addAPIError(diags, action, apiResp.StatusCode(), apiResp.JSONDefault)
		return false
	}
	if job := apiResp.JSON202.Data.Job; job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
			diags.AddError("Error waiting for gateway service reconfigure", err.Error())
			return false
		}
	}
	return true
}

func gatewayManagesConfig(config jsontypes.Normalized) bool {
	return !config.IsNull() && !config.IsUnknown()
}

func gatewayConfigFromValue(value jsontypes.Normalized) (*cycle.GatewayConfig, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var cfg cycle.GatewayConfig
	diags := value.Unmarshal(&cfg)
	if diags.HasError() {
		return nil, diags
	}
	return &cfg, nil
}

func gatewayConfigToValue(cfg *cycle.GatewayConfig) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cfg == nil {
		return jsontypes.NewNormalizedNull(), diags
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		diags.AddError("Error encoding gateway service config", err.Error())
		return jsontypes.NewNormalizedNull(), diags
	}
	if len(b) == 0 || string(b) == "null" {
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

func applyGatewayService(model *gatewayServiceResourceModel, svc *cycle.GatewayEnvironmentService, manageConfig bool) diag.Diagnostics {
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
		cfg, d := gatewayConfigToValue(svc.Config)
		diags.Append(d...)
		model.Config = cfg
	}

	return diags
}
