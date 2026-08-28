package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewPipelineTriggerKeyResource)
}

var (
	_ resource.Resource                = (*pipelineTriggerKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*pipelineTriggerKeyResource)(nil)
	_ resource.ResourceWithImportState = (*pipelineTriggerKeyResource)(nil)
)

// NewPipelineTriggerKeyResource returns the cycle_pipeline_trigger_key resource.
func NewPipelineTriggerKeyResource() resource.Resource {
	return &pipelineTriggerKeyResource{}
}

type pipelineTriggerKeyResource struct {
	client *CycleClient
}

type pipelineTriggerKeyResourceModel struct {
	ID         types.String `tfsdk:"id"`
	PipelineID types.String `tfsdk:"pipeline_id"`
	Name       types.String `tfsdk:"name"`
	IPs        types.List   `tfsdk:"ips"`
	Secret     types.String `tfsdk:"secret"`
	HubID      types.String `tfsdk:"hub_id"`
	State      types.String `tfsdk:"state"`
}

func (r *pipelineTriggerKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline_trigger_key"
}

func (r *pipelineTriggerKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle pipeline trigger key (`/v1/pipelines/{id}/keys`). Trigger keys " +
			"authenticate programmatic pipeline runs. The `secret` is returned only at create time; subsequent " +
			"reads keep the value already stored in state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the trigger key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"pipeline_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the pipeline this trigger key belongs to. Changing this forces a new trigger key to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A user-defined name for the trigger key.",
			},
			"ips": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "An allowlist of IP addresses that may use this trigger key. An empty list means no IP restriction is stored.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"secret": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The secret used when calling the trigger key programmatically. Returned on create; later reads preserve the value from state when the API omits it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this trigger key belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the trigger key (e.g. `live`).",
			},
		},
	}
}

func (r *pipelineTriggerKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *pipelineTriggerKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pipelineTriggerKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.CreatePipelineTriggerKeyJSONRequestBody{
		Name: plan.Name.ValueString(),
	}
	ips, d := pipelineTriggerKeyIPsToAPI(ctx, plan.IPs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Ips = ips

	createResp, err := r.client.Client.CreatePipelineTriggerKeyWithResponse(ctx, plan.PipelineID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating pipeline trigger key", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating pipeline trigger key", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(pipelineTriggerKeyModelFromAPI(ctx, &plan, createResp.JSON201.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pipelineTriggerKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pipelineTriggerKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetPipelineTriggerKeyWithResponse(ctx, state.PipelineID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading pipeline trigger key", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil || getResp.JSON200.Data == nil {
		addAPIError(&resp.Diagnostics, "reading pipeline trigger key", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	key := getResp.JSON200.Data
	if key.State.Current == cycle.TriggerKeyStateCurrentDeleted || key.State.Current == cycle.TriggerKeyStateCurrentDeleting {
		resp.State.RemoveResource(ctx)
		return
	}

	// The API typically omits `secret` after create. Keep the value already in state.
	resp.Diagnostics.Append(pipelineTriggerKeyModelFromAPI(ctx, &state, *key)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *pipelineTriggerKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan pipelineTriggerKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state pipelineTriggerKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	body := cycle.UpdatePipelineTriggerKeyJSONRequestBody{
		Name: &name,
	}
	ips, d := pipelineTriggerKeyIPsToAPI(ctx, plan.IPs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Ips = ips

	updateResp, err := r.client.Client.UpdatePipelineTriggerKeyWithResponse(ctx, state.PipelineID.ValueString(), state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating pipeline trigger key", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating pipeline trigger key", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	// Preserve the create-time secret unless the update response includes a new one.
	if !state.Secret.IsNull() && !state.Secret.IsUnknown() {
		plan.Secret = state.Secret
	}
	resp.Diagnostics.Append(pipelineTriggerKeyModelFromAPI(ctx, &plan, updateResp.JSON200.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pipelineTriggerKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pipelineTriggerKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeletePipelineTriggerKeyWithResponse(ctx, state.PipelineID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting pipeline trigger key", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting pipeline trigger key", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}
	if job := deleteResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for pipeline trigger key deletion", err.Error())
		}
	}
}

func (r *pipelineTriggerKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected an import ID in the form \"pipeline_id/trigger_key_id\", got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("pipeline_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func pipelineTriggerKeyIPsToAPI(ctx context.Context, list types.List) (*[]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	var ips []string
	diags.Append(list.ElementsAs(ctx, &ips, false)...)
	if diags.HasError() {
		return nil, diags
	}
	return &ips, diags
}

func pipelineTriggerKeyModelFromAPI(ctx context.Context, model *pipelineTriggerKeyResourceModel, key cycle.TriggerKey) diag.Diagnostics {
	var diags diag.Diagnostics

	priorSecret := model.Secret

	model.ID = types.StringValue(key.Id)
	model.PipelineID = types.StringValue(key.PipelineId)
	model.Name = types.StringValue(key.Name)
	model.HubID = types.StringValue(key.HubId)
	model.State = types.StringValue(string(key.State.Current))

	ips, d := types.ListValueFrom(ctx, types.StringType, stringSliceOrEmpty(key.Ips))
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	model.IPs = ips

	if key.Secret != "" {
		model.Secret = types.StringValue(key.Secret)
	} else if !priorSecret.IsNull() && !priorSecret.IsUnknown() && priorSecret.ValueString() != "" {
		model.Secret = priorSecret
	} else {
		model.Secret = types.StringNull()
	}

	return diags
}
