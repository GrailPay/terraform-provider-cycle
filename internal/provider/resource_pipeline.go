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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewPipelineResource)
}

var (
	_ resource.Resource                = (*pipelineResource)(nil)
	_ resource.ResourceWithConfigure   = (*pipelineResource)(nil)
	_ resource.ResourceWithImportState = (*pipelineResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*pipelineResource)(nil)
)

// NewPipelineResource returns the cycle_pipeline resource.
func NewPipelineResource() resource.Resource {
	return &pipelineResource{}
}

type pipelineResource struct {
	client *CycleClient
}

type pipelineResourceModel struct {
	ID         types.String         `tfsdk:"id"`
	Name       types.String         `tfsdk:"name"`
	Identifier types.String         `tfsdk:"identifier"`
	Disable    types.Bool           `tfsdk:"disable"`
	Dynamic    types.Bool           `tfsdk:"dynamic"`
	Stages     jsontypes.Normalized `tfsdk:"stages"`
	HubID      types.String         `tfsdk:"hub_id"`
	State      types.String         `tfsdk:"state"`
}

func (r *pipelineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline"
}

func (r *pipelineResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle pipeline (`/v1/pipelines`). Pipelines are ordered stages of steps " +
			"used to automate builds, deploys, and other hub operations.\n\n" +
			"`dynamic` is a **one-way** toggle: once enabled it cannot be set back to `false`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the pipeline.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A user-defined name for the pipeline.",
			},
			"identifier": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A human-readable slugged identifier for the pipeline. Automatically generated from the name if not provided.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"disable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, the pipeline cannot be triggered. Defaults to `false`.",
			},
			"dynamic": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, enables variable interpolation and other advanced logic on this pipeline. " +
					"This is a one-way toggle: once set to `true` it cannot be set back to `false`. Defaults to `false`.",
			},
			"stages": schema.StringAttribute{
				Optional:   true,
				CustomType: jsontypes.NormalizedType{},
				MarkdownDescription: "A JSON array of pipeline stages (each with `identifier`, optional `options`, and `steps`). " +
					"Modeled as normalized JSON because the step-type union is too large for a native schema. " +
					"Omit the attribute (or pass JSON `null` / `[]`) for a pipeline with no stages.",
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this pipeline belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the pipeline (e.g. `live`, `deleting`).",
			},
		},
	}
}

func (r *pipelineResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

// ModifyPlan rejects attempts to turn `dynamic` back off after it has been enabled.
func (r *pipelineResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state pipelineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan pipelineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if pipelineDynamicDisabledAfterEnabled(state.Dynamic, plan.Dynamic) {
		resp.Diagnostics.AddAttributeError(
			path.Root("dynamic"),
			"Cannot disable pipeline dynamic mode",
			"dynamic is a one-way toggle. Once set to true, it cannot be set back to false. "+
				"Create a new pipeline if you need a non-dynamic pipeline.",
		)
	}
}

func (r *pipelineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pipelineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := pipelineWriteBody(plan)
	if err != nil {
		resp.Diagnostics.AddError("Error building pipeline", err.Error())
		return
	}

	createResp, err := r.client.Client.CreatePipelineWithResponse(ctx, cycle.CreatePipelineJSONRequestBody{
		Name:       body.Name,
		Identifier: body.Identifier,
		Disable:    body.Disable,
		Dynamic:    body.Dynamic,
		Stages:     body.Stages,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating pipeline", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating pipeline", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(pipelineModelFromAPI(&plan, createResp.JSON201.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pipelineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pipelineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetPipelineWithResponse(ctx, state.ID.ValueString(), &cycle.GetPipelineParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading pipeline", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading pipeline", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	pipeline := getResp.JSON200.Data
	if pipeline.State.Current == cycle.PipelineStateCurrentDeleted || pipeline.State.Current == cycle.PipelineStateCurrentDeleting {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(pipelineModelFromAPI(&state, pipeline)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *pipelineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan pipelineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state pipelineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if pipelineDynamicDisabledAfterEnabled(state.Dynamic, plan.Dynamic) {
		resp.Diagnostics.AddAttributeError(
			path.Root("dynamic"),
			"Cannot disable pipeline dynamic mode",
			"dynamic is a one-way toggle. Once set to true, it cannot be set back to false.",
		)
		return
	}

	body, err := pipelineWriteBody(plan)
	if err != nil {
		resp.Diagnostics.AddError("Error building pipeline", err.Error())
		return
	}

	name := body.Name
	updateResp, err := r.client.Client.UpdatePipelineWithResponse(ctx, state.ID.ValueString(), cycle.UpdatePipelineJSONRequestBody{
		Name:       &name,
		Identifier: body.Identifier,
		Disable:    body.Disable,
		Dynamic:    body.Dynamic,
		Stages:     body.Stages,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating pipeline", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating pipeline", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(pipelineModelFromAPI(&plan, updateResp.JSON200.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pipelineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pipelineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeletePipelineWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting pipeline", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	// The generated client maps a successful delete to JSON200 (JobDescriptor),
	// even though some Cycle docs describe this as HTTP 202.
	if deleteResp.JSON200 != nil {
		if job := deleteResp.JSON200.Data.Job; job != nil {
			if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
				resp.Diagnostics.AddError("Error waiting for pipeline deletion", err.Error())
			}
		}
		return
	}
	if deleteResp.StatusCode() == http.StatusAccepted {
		return
	}
	addAPIError(&resp.Diagnostics, "deleting pipeline", deleteResp.StatusCode(), deleteResp.JSONDefault)
}

func (r *pipelineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

type pipelineWriteFields struct {
	Name       string
	Identifier *string
	Disable    *bool
	Dynamic    *bool
	Stages     *[]cycle.PipelineStage
}

func pipelineWriteBody(plan pipelineResourceModel) (pipelineWriteFields, error) {
	disable := plan.Disable.ValueBool()
	dynamic := plan.Dynamic.ValueBool()
	body := pipelineWriteFields{
		Name:    plan.Name.ValueString(),
		Disable: &disable,
		Dynamic: &dynamic,
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}

	stages, err := pipelineStagesToAPI(plan.Stages)
	if err != nil {
		return pipelineWriteFields{}, err
	}
	body.Stages = stages
	return body, nil
}

func pipelineDynamicDisabledAfterEnabled(state, plan types.Bool) bool {
	return state.ValueBool() && !plan.IsNull() && !plan.IsUnknown() && !plan.ValueBool()
}

func pipelineModelFromAPI(model *pipelineResourceModel, pipeline cycle.Pipeline) diag.Diagnostics {
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

func pipelineStagesToAPI(stages jsontypes.Normalized) (*[]cycle.PipelineStage, error) {
	if stages.IsNull() || stages.IsUnknown() {
		return nil, nil
	}
	raw := stages.ValueString()
	if raw == "" || raw == "null" {
		return nil, nil
	}

	var out []cycle.PipelineStage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("stages must be a JSON array of pipeline stages: %w", err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out, nil
}

func pipelineStagesFromAPI(stages *[]cycle.PipelineStage) (jsontypes.Normalized, error) {
	if stages == nil || len(*stages) == 0 {
		return jsontypes.NewNormalizedNull(), nil
	}
	b, err := json.Marshal(*stages)
	if err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	return jsontypes.NewNormalizedValue(string(b)), nil
}
