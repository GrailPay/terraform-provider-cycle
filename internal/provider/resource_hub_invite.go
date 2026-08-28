package provider

import (
	"context"
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
	RegisterResource(NewHubInviteResource)
}

var (
	_ resource.Resource                = (*hubInviteResource)(nil)
	_ resource.ResourceWithConfigure   = (*hubInviteResource)(nil)
	_ resource.ResourceWithImportState = (*hubInviteResource)(nil)
)

// NewHubInviteResource returns the cycle_hub_invite resource.
func NewHubInviteResource() resource.Resource {
	return &hubInviteResource{}
}

type hubInviteResource struct {
	client *CycleClient
}

type hubInviteResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Recipient types.String `tfsdk:"recipient"`
	RoleID    types.String `tfsdk:"role_id"`
	State     types.String `tfsdk:"state"`
}

func (r *hubInviteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub_invite"
}

func (r *hubInviteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sends an invitation to join the current Cycle hub (`/v1/hubs/current/invites`). Invites cannot be modified after they are sent, so any change forces a new invite. Once the recipient accepts (or declines) the invite, it disappears from the hub's pending invites and this resource is removed from state on the next refresh; use `cycle_hub_member` to manage the resulting membership.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the invite (a pending hub membership ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"recipient": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The email address of the person to invite. Changing this forces a new invite.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the hub role the invitee will be assigned upon accepting. Changing this forces a new invite.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the invite membership (e.g. `pending`).",
			},
		},
	}
}

func (r *hubInviteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func setInviteState(membership *cycle.HubMembership, m *hubInviteResourceModel) {
	m.ID = types.StringValue(membership.Id)
	m.Recipient = types.StringValue(membership.Invitation.Recipient)
	m.RoleID = types.StringValue(membership.RoleId)
	m.State = types.StringValue(string(membership.State.Current))
}

func (r *hubInviteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan hubInviteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recipient := plan.Recipient.ValueString()
	roleID := plan.RoleID.ValueString()
	body := cycle.CreateHubInviteJSONRequestBody{
		Recipient: &recipient,
		RoleId:    &roleID,
	}

	apiResp, err := r.client.Client.CreateHubInviteWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating hub invite", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating hub invite", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	membership := apiResp.JSON201.Data
	setInviteState(&membership, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// findInvite scans the hub's pending invites for the given membership ID.
// The API exposes no single-invite GET endpoint, so we paginate the list.
func (r *hubInviteResource) findInvite(ctx context.Context, id string) (*cycle.HubMembership, error) {
	const pageSize = 100
	for page := float32(1); ; page++ {
		size := float32(pageSize)
		number := page
		apiResp, err := r.client.Client.GetHubInvitesWithResponse(ctx, &cycle.GetHubInvitesParams{
			Page: &cycle.PageParam{Number: &number, Size: &size},
		})
		if err != nil {
			return nil, err
		}
		if apiResp.JSON200 == nil {
			return nil, apiError("listing hub invites", apiResp.StatusCode(), apiResp.JSONDefault)
		}

		for i := range apiResp.JSON200.Data {
			if apiResp.JSON200.Data[i].Id == id {
				return &apiResp.JSON200.Data[i], nil
			}
		}
		if len(apiResp.JSON200.Data) < pageSize {
			return nil, nil
		}
	}
}

func (r *hubInviteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state hubInviteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	membership, err := r.findInvite(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading hub invite", err.Error())
		return
	}
	// Invite no longer pending: it was accepted, declined, revoked, or
	// expired. In all cases the invite itself is gone.
	if membership == nil || membership.State.Current != cycle.MembershipStateCurrentPending {
		resp.State.RemoveResource(ctx)
		return
	}

	setInviteState(membership, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called: every user-settable attribute requires replacement.
func (r *hubInviteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan hubInviteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hubInviteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}

	var state hubInviteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.DeleteHubInviteWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting hub invite", err.Error())
		return
	}
	// Already accepted/declined/revoked invites are gone; treat as deleted.
	if apiResp.StatusCode() == http.StatusNotFound {
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "deleting hub invite", apiResp.StatusCode(), apiResp.JSONDefault)
	}
}

func (r *hubInviteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
