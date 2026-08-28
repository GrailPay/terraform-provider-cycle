package provider

import (
	"context"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewClusterResource)
}

var (
	_ resource.Resource                = (*clusterResource)(nil)
	_ resource.ResourceWithConfigure   = (*clusterResource)(nil)
	_ resource.ResourceWithImportState = (*clusterResource)(nil)
)

// NewClusterResource returns the cycle_cluster resource.
func NewClusterResource() resource.Resource {
	return &clusterResource{}
}

type clusterResource struct {
	client *CycleClient
}

type clusterResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Identifier   types.String `tfsdk:"identifier"`
	NonEssential types.Bool   `tfsdk:"non_essential"`
	HubID        types.String `tfsdk:"hub_id"`
	State        types.String `tfsdk:"state"`
}

func (r *clusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (r *clusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle infrastructure cluster. Clusters are groups of servers that allow physical separation of resources; environments are deployed into a cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the cluster.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"identifier": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A human-readable slugged identifier for the cluster (e.g. `production`). Changing this forces a new cluster to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"non_essential": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Marks the cluster as non-essential. Non-essential cluster resources are excluded by default from certain metrics and summaries unless opted in. Defaults to `false`.",
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this cluster belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the cluster (e.g. `live`).",
			},
		},
	}
}

func (r *clusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *clusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan clusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.Client.CreateClusterWithResponse(ctx, cycle.CreateClusterJSONRequestBody{
		Identifier: plan.Identifier.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating cluster", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating cluster", createResp.StatusCode(), createResp.JSONDefault)
		return
	}
	cluster := createResp.JSON201.Data

	// The create endpoint only accepts an identifier; non_essential is applied
	// with a follow-up update.
	if plan.NonEssential.ValueBool() {
		nonEssential := true
		updateResp, err := r.client.Client.UpdateClusterWithResponse(ctx, cluster.Id, cycle.UpdateClusterJSONRequestBody{
			NonEssential: &nonEssential,
		})
		if err != nil {
			clusterModelFromAPI(&plan, cluster)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			resp.Diagnostics.AddError("Error updating cluster after create", err.Error())
			return
		}
		if updateResp.JSON200 == nil {
			clusterModelFromAPI(&plan, cluster)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			addAPIError(&resp.Diagnostics, "updating cluster after create", updateResp.StatusCode(), updateResp.JSONDefault)
			return
		}
		cluster = updateResp.JSON200.Data
	}

	clusterModelFromAPI(&plan, cluster)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *clusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state clusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetClusterWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading cluster", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	clusterModelFromAPI(&state, getResp.JSON200.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *clusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan clusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state clusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nonEssential := plan.NonEssential.ValueBool()
	updateResp, err := r.client.Client.UpdateClusterWithResponse(ctx, state.ID.ValueString(), cycle.UpdateClusterJSONRequestBody{
		NonEssential: &nonEssential,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating cluster", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating cluster", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	clusterModelFromAPI(&plan, updateResp.JSON200.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *clusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state clusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteClusterWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting cluster", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting cluster", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}

	if job := deleteResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for cluster deletion", err.Error())
		}
	}
}

func (r *clusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func clusterModelFromAPI(model *clusterResourceModel, cluster cycle.Cluster) {
	model.ID = types.StringValue(cluster.Id)
	model.Identifier = types.StringValue(cluster.Identifier)
	model.NonEssential = types.BoolValue(cluster.NonEssential)
	model.HubID = types.StringValue(cluster.HubId)
	model.State = types.StringValue(string(cluster.State.Current))
}
