package provider

import (
	"context"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewVPNResource)
}

var (
	_ resource.Resource                = (*vpnResource)(nil)
	_ resource.ResourceWithConfigure   = (*vpnResource)(nil)
	_ resource.ResourceWithImportState = (*vpnResource)(nil)
)

// NewVPNResource returns the cycle_vpn resource.
func NewVPNResource() resource.Resource {
	return &vpnResource{}
}

type vpnResource struct {
	client *CycleClient
}

type vpnResourceModel struct {
	ID               types.String `tfsdk:"id"`
	EnvironmentID    types.String `tfsdk:"environment_id"`
	Enable           types.Bool   `tfsdk:"enable"`
	HighAvailability types.Bool   `tfsdk:"high_availability"`
	AutoUpdate       types.Bool   `tfsdk:"auto_update"`
	AllowInternet    types.Bool   `tfsdk:"allow_internet"`
	CycleAccounts    types.Bool   `tfsdk:"cycle_accounts"`
	VPNAccounts      types.Bool   `tfsdk:"vpn_accounts"`
	Webhook          types.String `tfsdk:"webhook"`
	CustomDirectives types.String `tfsdk:"custom_directives"`
	URL              types.String `tfsdk:"url"`
}

func (r *vpnResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpn"
}

func (r *vpnResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the VPN service for a Cycle environment. The VPN is an environment singleton — it always exists and has no create or delete HTTP resource. Create and update send a `reconfigure` job; destroy disables the service and removes the resource from state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The environment ID this VPN belongs to. The VPN is a singleton keyed on `environment_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose VPN is managed. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the VPN service is enabled.",
			},
			"high_availability": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the VPN service runs in high availability mode. The GET VPN service payload does not return this field; Terraform keeps the last configured value after apply.",
			},
			"auto_update": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the VPN service container is set to auto-update.",
			},
			"allow_internet": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "When `true`, routes all traffic through the VPN, including non-Cycle traffic.",
			},
			"cycle_accounts": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "When `true`, any Cycle account with access to the environment can log in to the VPN with their Cycle email and password.",
			},
			"vpn_accounts": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "When `true`, custom VPN user accounts (see `cycle_vpn_user`) can log in to the VPN.",
			},
			"webhook": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional webhook URL used to authorize VPN logins. Cycle posts the supplied credentials and expects HTTP 200 to permit the login.",
			},
			"custom_directives": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Additional OpenVPN directives appended to the server configuration on service start. Each line should follow standard OpenVPN syntax.",
			},
			"url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The URL associated with the VPN service.",
			},
		},
	}
}

func (r *vpnResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *vpnResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan vpnResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureVPN(ctx, plan, &resp.Diagnostics, "creating VPN") {
		return
	}
	if !r.refreshVPN(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vpnResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state vpnResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	info, status, envelope, err := r.getVPN(ctx, state.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading VPN", err.Error())
		return
	}
	if status == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if info == nil {
		addAPIError(&resp.Diagnostics, "reading VPN", status, envelope)
		return
	}

	applyVPNInfo(&state, *info)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vpnResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		return
	}

	var plan vpnResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.reconfigureVPN(ctx, plan, &resp.Diagnostics, "updating VPN") {
		return
	}
	if !r.refreshVPN(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vpnResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}

	var state vpnResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Enable = types.BoolValue(false)
	state.HighAvailability = types.BoolValue(false)
	state.AutoUpdate = types.BoolValue(false)
	if !r.reconfigureVPN(ctx, state, &resp.Diagnostics, "disabling VPN") {
		return
	}
}

func (r *vpnResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *vpnResource) getVPN(ctx context.Context, environmentID string) (*cycle.VPNInfoReturn, int, *cycle.ErrorEnvelope, error) {
	apiResp, err := r.client.Client.GetVPNServiceWithResponse(ctx, environmentID)
	if err != nil {
		return nil, 0, nil, err
	}
	if apiResp.JSON200 == nil {
		return nil, apiResp.StatusCode(), apiResp.JSONDefault, nil
	}
	return &apiResp.JSON200.Data, apiResp.StatusCode(), nil, nil
}

func (r *vpnResource) refreshVPN(ctx context.Context, model *vpnResourceModel, diags *diag.Diagnostics) bool {
	info, status, envelope, err := r.getVPN(ctx, model.EnvironmentID.ValueString())
	if err != nil {
		diags.AddError("Error reading VPN", err.Error())
		return false
	}
	if info == nil {
		addAPIError(diags, "reading VPN", status, envelope)
		return false
	}
	applyVPNInfo(model, *info)
	return true
}

func (r *vpnResource) reconfigureVPN(ctx context.Context, model vpnResourceModel, diags *diag.Diagnostics, action string) bool {
	actionBody := cycle.VpnReconfigureAction{
		Action: cycle.VpnReconfigureActionActionReconfigure,
	}
	actionBody.Contents.Enable = servicesBoolPtr(model.Enable)
	actionBody.Contents.HighAvailability = servicesBoolPtr(model.HighAvailability)
	actionBody.Contents.AutoUpdate = servicesBoolPtr(model.AutoUpdate)
	actionBody.Contents.Config = vpnConfigFromModel(model)

	var task cycle.VpnTask
	if err := task.FromVpnReconfigureAction(actionBody); err != nil {
		diags.AddError("Error building VPN reconfigure task", err.Error())
		return false
	}

	apiResp, err := r.client.Client.CreateVPNServiceJobWithResponse(ctx, model.EnvironmentID.ValueString(), task)
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
			diags.AddError("Error waiting for VPN reconfigure", err.Error())
			return false
		}
	}
	return true
}

func vpnConfigFromModel(model vpnResourceModel) *struct {
	AllowInternet *bool `json:"allow_internet,omitempty"`
	Auth          *struct {
		CycleAccounts bool    `json:"cycle_accounts"`
		VpnAccounts   *bool   `json:"vpn_accounts,omitempty"`
		Webhook       *string `json:"webhook"`
	} `json:"auth,omitempty"`
	CustomDirectives *string `json:"custom_directives,omitempty"`
} {
	allowInternet := servicesBoolPtr(model.AllowInternet)
	cycleAccounts := model.CycleAccounts.ValueBool()
	vpnAccounts := servicesBoolPtr(model.VPNAccounts)
	webhook := servicesStringPtr(model.Webhook)
	custom := servicesStringPtr(model.CustomDirectives)

	return &struct {
		AllowInternet *bool `json:"allow_internet,omitempty"`
		Auth          *struct {
			CycleAccounts bool    `json:"cycle_accounts"`
			VpnAccounts   *bool   `json:"vpn_accounts,omitempty"`
			Webhook       *string `json:"webhook"`
		} `json:"auth,omitempty"`
		CustomDirectives *string `json:"custom_directives,omitempty"`
	}{
		AllowInternet: allowInternet,
		Auth: &struct {
			CycleAccounts bool    `json:"cycle_accounts"`
			VpnAccounts   *bool   `json:"vpn_accounts,omitempty"`
			Webhook       *string `json:"webhook"`
		}{
			CycleAccounts: cycleAccounts,
			VpnAccounts:   vpnAccounts,
			Webhook:       webhook,
		},
		CustomDirectives: custom,
	}
}

func applyVPNInfo(model *vpnResourceModel, info cycle.VPNInfoReturn) {
	model.ID = types.StringValue(model.EnvironmentID.ValueString())
	model.URL = types.StringValue(info.Url)

	// VpnEnvironmentService has no high_availability field; keep the last
	// configured value (or false after import).
	if model.HighAvailability.IsNull() || model.HighAvailability.IsUnknown() {
		model.HighAvailability = types.BoolValue(false)
	}

	if info.Service == nil {
		model.Enable = types.BoolValue(false)
		model.AutoUpdate = types.BoolValue(false)
		model.AllowInternet = types.BoolValue(false)
		model.CycleAccounts = types.BoolValue(false)
		model.VPNAccounts = types.BoolValue(false)
		model.Webhook = types.StringNull()
		model.CustomDirectives = types.StringNull()
		return
	}

	svc := info.Service
	model.Enable = types.BoolValue(svc.Enable)
	if svc.AutoUpdate != nil {
		model.AutoUpdate = types.BoolValue(*svc.AutoUpdate)
	} else {
		model.AutoUpdate = types.BoolValue(false)
	}

	if svc.Config == nil {
		model.AllowInternet = types.BoolValue(false)
		model.CycleAccounts = types.BoolValue(false)
		model.VPNAccounts = types.BoolValue(false)
		model.Webhook = types.StringNull()
		model.CustomDirectives = types.StringNull()
		return
	}

	model.AllowInternet = types.BoolValue(svc.Config.AllowInternet)
	model.CycleAccounts = types.BoolValue(svc.Config.Auth.CycleAccounts)
	if svc.Config.Auth.VpnAccounts != nil {
		model.VPNAccounts = types.BoolValue(*svc.Config.Auth.VpnAccounts)
	} else {
		model.VPNAccounts = types.BoolValue(false)
	}
	if svc.Config.Auth.Webhook != nil && *svc.Config.Auth.Webhook != "" {
		model.Webhook = types.StringValue(*svc.Config.Auth.Webhook)
	} else {
		model.Webhook = types.StringNull()
	}
	if svc.Config.CustomDirectives != nil && *svc.Config.CustomDirectives != "" {
		model.CustomDirectives = types.StringValue(*svc.Config.CustomDirectives)
	} else {
		model.CustomDirectives = types.StringNull()
	}
}

func servicesStringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}
