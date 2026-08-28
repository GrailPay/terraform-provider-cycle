package provider

import (
	"context"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewHubMemberResource)
}

var (
	_ resource.Resource                = (*hubMemberResource)(nil)
	_ resource.ResourceWithConfigure   = (*hubMemberResource)(nil)
	_ resource.ResourceWithImportState = (*hubMemberResource)(nil)
)

// NewHubMemberResource returns the cycle_hub_member resource.
func NewHubMemberResource() resource.Resource {
	return &hubMemberResource{}
}

type hubMemberResource struct {
	client *CycleClient
}

type hubMemberResourceModel struct {
	ID        types.String `tfsdk:"id"`
	AccountID types.String `tfsdk:"account_id"`
	RoleID    types.String `tfsdk:"role_id"`
	State     types.String `tfsdk:"state"`
}

func (r *hubMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub_member"
}

func (r *hubMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the role of an existing member of the current Cycle hub (`/v1/hubs/current/members`). " +
			"The Cycle API has no endpoint to directly create a membership — members join by accepting an invite (see `cycle_hub_invite`). " +
			"On create, this resource adopts the existing membership for the given account and sets its role. " +
			"Destroying this resource removes the member from the hub.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the hub membership.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"account_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the Cycle account whose hub membership is managed. The account must already be a member of the hub (i.e. have accepted an invite). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the hub role assigned to this member.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the membership (e.g. `accepted`).",
			},
		},
	}
}

func (r *hubMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func setMemberState(membership *cycle.HubMembership, m *hubMemberResourceModel) {
	m.ID = types.StringValue(membership.Id)
	if membership.AccountId != nil {
		m.AccountID = types.StringValue(*membership.AccountId)
	}
	m.RoleID = types.StringValue(membership.RoleId)
	m.State = types.StringValue(string(membership.State.Current))
}

// patchMemberRole assigns roleID to the membership and returns the updated
// membership.
func (r *hubMemberResource) patchMemberRole(ctx context.Context, memberID, roleID string) (*cycle.HubMembership, error) {
	apiResp, err := r.client.Client.UpdateHubMemberWithResponse(ctx, memberID, cycle.UpdateHubMemberJSONRequestBody{
		RoleId: &roleID,
	})
	if err != nil {
		return nil, err
	}
	if apiResp.JSON200 == nil {
		return nil, apiError("updating hub member role", apiResp.StatusCode(), apiResp.JSONDefault)
	}
	return &apiResp.JSON200.Data, nil
}

// Create adopts the existing hub membership for the configured account and
// patches its role. There is no POST /v1/hubs/current/members endpoint;
// memberships only come into existence when an account accepts an invite.
func (r *hubMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan hubMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := plan.AccountID.ValueString()
	lookup, err := r.client.Client.GetHubMemberAccountWithResponse(ctx, accountID, &cycle.GetHubMemberAccountParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error creating hub member", err.Error())
		return
	}
	if lookup.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Error creating hub member",
			fmt.Sprintf("Account %s has no membership on this hub. Cycle has no API to add a member directly; "+
				"invite the account first (e.g. with cycle_hub_invite) and have them accept before managing their membership.", accountID),
		)
		return
	}
	if lookup.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "creating hub member", lookup.StatusCode(), lookup.JSONDefault)
		return
	}

	membership, err := r.patchMemberRole(ctx, lookup.JSON200.Data.Id, plan.RoleID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating hub member", err.Error())
		return
	}

	setMemberState(membership, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hubMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state hubMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.GetHubMemberWithResponse(ctx, state.ID.ValueString(), &cycle.GetHubMemberParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading hub member", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading hub member", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	membership := apiResp.JSON200.Data
	if membership.State.Current == cycle.MembershipStateCurrentDeleted {
		resp.State.RemoveResource(ctx)
		return
	}

	setMemberState(&membership, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *hubMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		return
	}

	var plan hubMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state hubMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	membership, err := r.patchMemberRole(ctx, state.ID.ValueString(), plan.RoleID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error updating hub member", err.Error())
		return
	}

	setMemberState(membership, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hubMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}

	var state hubMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.DeleteHubMemberWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting hub member", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return
	}
	if apiResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting hub member", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	if job := apiResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error deleting hub member", err.Error())
		}
	}
}

func (r *hubMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
