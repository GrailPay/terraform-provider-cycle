package provider

import (
	"context"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewEnvironmentResource)
}

var (
	_ resource.Resource                = (*environmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*environmentResource)(nil)
	_ resource.ResourceWithImportState = (*environmentResource)(nil)
)

// NewEnvironmentResource returns the cycle_environment resource.
func NewEnvironmentResource() resource.Resource {
	return &environmentResource{}
}

type environmentResource struct {
	client *CycleClient
}

type environmentResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Identifier       types.String `tfsdk:"identifier"`
	Cluster          types.String `tfsdk:"cluster"`
	Description      types.String `tfsdk:"description"`
	LegacyNetworking types.Bool   `tfsdk:"legacy_networking"`
	HubID            types.String `tfsdk:"hub_id"`
	State            types.String `tfsdk:"state"`
}

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle environment. Environments are groups of containers with a private network built between them, deployed into a cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the environment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A user-defined name for the environment.",
			},
			"identifier": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A human-readable slugged identifier for the environment. Automatically generated from the name if not provided.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The identifier of the cluster this environment is deployed into. Changing this forces a new environment to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "A custom description for the environment.",
			},
			"legacy_networking": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether legacy networking mode is enabled on this environment. Can only be set at creation time; changing this forces a new environment to be created. Defaults to `false`.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this environment belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the environment (e.g. `live`).",
			},
		},
	}
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.CreateEnvironmentJSONRequestBody{
		Name:    plan.Name.ValueString(),
		Cluster: plan.Cluster.ValueString(),
		About: struct {
			Description string `json:"description"`
		}{
			Description: plan.Description.ValueString(),
		},
		Features: cycle.EnvironmentFeatures{
			LegacyNetworking: plan.LegacyNetworking.ValueBool(),
		},
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}

	createResp, err := r.client.Client.CreateEnvironmentWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating environment", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating environment", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	environmentModelFromAPI(&plan, createResp.JSON201.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetEnvironmentWithResponse(ctx, state.ID.ValueString(), &cycle.GetEnvironmentParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading environment", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading environment", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	environmentModelFromAPI(&state, getResp.JSON200.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan environmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state environmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The about object requires the favorite flag, which this resource does
	// not manage; fetch the current value so the update preserves it.
	favorite := false
	getResp, err := r.client.Client.GetEnvironmentWithResponse(ctx, state.ID.ValueString(), &cycle.GetEnvironmentParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading environment before update", err.Error())
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading environment before update", getResp.StatusCode(), getResp.JSONDefault)
		return
	}
	favorite = getResp.JSON200.Data.About.Favorite

	name := plan.Name.ValueString()
	body := cycle.UpdateEnvironmentJSONRequestBody{
		Name: &name,
		About: &cycle.EnvironmentAbout{
			Description: plan.Description.ValueString(),
			Favorite:    favorite,
		},
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}

	updateResp, err := r.client.Client.UpdateEnvironmentWithResponse(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating environment", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating environment", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	environmentModelFromAPI(&plan, updateResp.JSON200.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteEnvironmentWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting environment", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting environment", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}

	// Environment deletion is asynchronous; wait for the job so dependent
	// resources (e.g. the cluster) can be deleted afterwards.
	if job := deleteResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for environment deletion", err.Error())
		}
	}
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func environmentModelFromAPI(model *environmentResourceModel, env cycle.Environment) {
	model.ID = types.StringValue(env.Id)
	model.Name = types.StringValue(env.Name)
	model.Identifier = types.StringValue(env.Identifier)
	model.Cluster = types.StringValue(env.Cluster)
	model.Description = types.StringValue(env.About.Description)
	model.LegacyNetworking = types.BoolValue(env.Features.LegacyNetworking)
	model.HubID = types.StringValue(env.HubId)
	model.State = types.StringValue(string(env.State.Current))
}
