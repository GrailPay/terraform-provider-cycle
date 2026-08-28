package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewVPNUserResource)
}

var (
	_ resource.Resource                = (*vpnUserResource)(nil)
	_ resource.ResourceWithConfigure   = (*vpnUserResource)(nil)
	_ resource.ResourceWithImportState = (*vpnUserResource)(nil)
)

// NewVPNUserResource returns the cycle_vpn_user resource.
func NewVPNUserResource() resource.Resource {
	return &vpnUserResource{}
}

type vpnUserResource struct {
	client *CycleClient
}

type vpnUserResourceModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`
	LastLogin     types.String `tfsdk:"last_login"`
}

func (r *vpnUserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpn_user"
}

func (r *vpnUserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom VPN user on an environment's VPN service. There is no update endpoint — changing `environment_id`, `username`, or `password` forces a new user to be created.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the VPN user.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose VPN this user belongs to. Changing this forces a new VPN user to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The VPN login username. Changing this forces a new VPN user to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				WriteOnly:           true,
				MarkdownDescription: "The VPN login password. Write-only and never returned by the API. Changing this forces a new VPN user to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"last_login": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp of the last time this user logged into the VPN, if any.",
			},
		},
	}
}

func (r *vpnUserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *vpnUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan vpnUserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.CreateVPNUserJSONRequestBody{
		Username: plan.Username.ValueString(),
		Password: plan.Password.ValueString(),
	}

	apiResp, err := r.client.Client.CreateVPNUserWithResponse(ctx, plan.EnvironmentID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating VPN user", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating VPN user", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	vpnUserModelFromAPI(&plan, apiResp.JSON201.Data)
	plan.Password = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vpnUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state vpnUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, status, envelope, err := r.findVPNUser(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Error reading VPN user", err.Error())
		return
	}
	if status == http.StatusNotFound || user == nil && envelope == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if user == nil {
		addAPIError(&resp.Diagnostics, "reading VPN user", status, envelope)
		return
	}

	// The API never returns the password. Write-only attributes are omitted
	// from state; if the framework still has a prior value, keep it.
	vpnUserModelFromAPI(&state, *user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vpnUserResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"VPN users cannot be updated",
		"The Cycle API has no endpoint to modify a VPN user. Changing username or password forces a new resource.",
	)
}

func (r *vpnUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}

	var state vpnUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := state.ID.ValueString()
	if userID == "" {
		user, status, envelope, err := r.findVPNUser(ctx, state)
		if err != nil {
			resp.Diagnostics.AddError("Error looking up VPN user before delete", err.Error())
			return
		}
		if status == http.StatusNotFound || user == nil && envelope == nil {
			return
		}
		if user == nil {
			addAPIError(&resp.Diagnostics, "looking up VPN user before delete", status, envelope)
			return
		}
		userID = user.Id
	}

	apiResp, err := r.client.Client.DeleteVPNUserWithResponse(ctx, state.EnvironmentID.ValueString(), userID)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting VPN user", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "deleting VPN user", apiResp.StatusCode(), apiResp.JSONDefault)
	}
}

// ImportState imports a VPN user using "environment_id/username" or
// "environment_id/id".
func (r *vpnUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected an import ID in the form \"environment_id/username\" or \"environment_id/id\", got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[0])...)
	if vpnUserLooksLikeObjectID(parts[1]) {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	} else {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("username"), parts[1])...)
	}
}

func (r *vpnUserResource) findVPNUser(ctx context.Context, state vpnUserResourceModel) (*cycle.VPNUsers, int, *cycle.ErrorEnvelope, error) {
	apiResp, err := r.client.Client.GetVPNUsersWithResponse(ctx, state.EnvironmentID.ValueString())
	if err != nil {
		return nil, 0, nil, err
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return nil, apiResp.StatusCode(), nil, nil
	}
	if apiResp.JSON200 == nil {
		return nil, apiResp.StatusCode(), apiResp.JSONDefault, nil
	}

	id := state.ID.ValueString()
	username := state.Username.ValueString()
	for i := range apiResp.JSON200.Data {
		user := apiResp.JSON200.Data[i]
		if id != "" && user.Id == id {
			return &user, apiResp.StatusCode(), nil, nil
		}
	}
	if username != "" {
		for i := range apiResp.JSON200.Data {
			user := apiResp.JSON200.Data[i]
			if user.Username == username {
				return &user, apiResp.StatusCode(), nil, nil
			}
		}
	}

	return nil, apiResp.StatusCode(), nil, nil
}

func vpnUserModelFromAPI(model *vpnUserResourceModel, user cycle.VPNUsers) {
	model.ID = types.StringValue(user.Id)
	model.EnvironmentID = types.StringValue(user.EnvironmentId)
	model.Username = types.StringValue(user.Username)
	if user.LastLogin.IsZero() {
		model.LastLogin = types.StringNull()
	} else {
		model.LastLogin = types.StringValue(user.LastLogin.UTC().Format(time.RFC3339))
	}
}

func vpnUserLooksLikeObjectID(s string) bool {
	if len(s) != 24 {
		return false
	}
	for _, r := range s {
		if !unicode.Is(unicode.ASCII_Hex_Digit, r) {
			return false
		}
	}
	return true
}
