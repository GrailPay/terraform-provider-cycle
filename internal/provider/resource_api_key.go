package provider

import (
	"context"
	"net/http"

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
	RegisterResource(NewAPIKeyResource)
}

var (
	_ resource.Resource                = (*apiKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*apiKeyResource)(nil)
	_ resource.ResourceWithImportState = (*apiKeyResource)(nil)
)

// NewAPIKeyResource returns the cycle_api_key resource.
func NewAPIKeyResource() resource.Resource {
	return &apiKeyResource{}
}

type apiKeyResource struct {
	client *CycleClient
}

type apiKeyResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	RoleID types.String `tfsdk:"role_id"`
	IPs    types.List   `tfsdk:"ips"`
	Secret types.String `tfsdk:"secret"`
	HubID  types.String `tfsdk:"hub_id"`
	State  types.String `tfsdk:"state"`
}

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle hub API key (`/v1/api-keys`). Permissions come from `role_id`; there is " +
			"no separate capabilities field. The `secret` is returned only at create time; subsequent reads keep " +
			"the value already stored in state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the API key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A user-defined name for the API key.",
			},
			"role_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the hub role this API key inherits permissions from.",
			},
			"ips": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "An allowlist of IP addresses that may use this API key. An empty list means the key is unrestricted.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"secret": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The API key secret. Returned on create; later reads preserve the value from state when the API omits it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this API key belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the API key (e.g. `live`).",
			},
		},
	}
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.CreateApiKeyJSONRequestBody{
		Name:   plan.Name.ValueString(),
		RoleId: plan.RoleID.ValueString(),
	}
	ips, d := apiKeyIPsToAPI(ctx, plan.IPs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Ips = ips

	createResp, err := r.client.Client.CreateApiKeyWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating API key", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating API key", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(apiKeyModelFromAPI(ctx, &plan, createResp.JSON201.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetAPIKeyWithResponse(ctx, state.ID.ValueString(), &cycle.GetAPIKeyParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading API key", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading API key", getResp.StatusCode(), getResp.JSONDefault)
		return
	}
	if getResp.JSON200.Data == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	key := getResp.JSON200.Data
	if apiKeyStateDeleted(key.State.Current) {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(apiKeyModelFromAPI(ctx, &state, *key)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	roleID := plan.RoleID.ValueString()
	body := cycle.UpdateAPIKeyJSONRequestBody{
		Name:   &name,
		RoleId: &roleID,
	}
	ips, d := apiKeyIPsToAPI(ctx, plan.IPs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Ips = ips

	updateResp, err := r.client.Client.UpdateAPIKeyWithResponse(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating API key", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating API key", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	if !state.Secret.IsNull() && !state.Secret.IsUnknown() {
		plan.Secret = state.Secret
	}
	resp.Diagnostics.Append(apiKeyModelFromAPI(ctx, &plan, updateResp.JSON200.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteAPIKeyWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting API key", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "deleting API key", deleteResp.StatusCode(), deleteResp.JSONDefault)
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func apiKeyStateDeleted(current cycle.ApiKeyStateCurrent) bool {
	return current == cycle.ApiKeyStateCurrentDeleted || current == cycle.ApiKeyStateCurrentDeleting
}

func apiKeyIPsToAPI(ctx context.Context, list types.List) (*[]string, diag.Diagnostics) {
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

func apiKeyModelFromAPI(ctx context.Context, model *apiKeyResourceModel, key cycle.ApiKey) diag.Diagnostics {
	var diags diag.Diagnostics

	priorSecret := model.Secret

	model.ID = types.StringValue(key.Id)
	model.Name = types.StringValue(key.Name)
	model.RoleID = types.StringValue(key.RoleId)
	model.HubID = types.StringValue(key.HubId)
	model.State = types.StringValue(string(key.State.Current))

	var ips []string
	if key.Ips != nil {
		ips = *key.Ips
	}
	list, d := types.ListValueFrom(ctx, types.StringType, stringSliceOrEmpty(ips))
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	model.IPs = list

	if key.Secret != "" {
		model.Secret = types.StringValue(key.Secret)
	} else if !priorSecret.IsNull() && !priorSecret.IsUnknown() && priorSecret.ValueString() != "" {
		model.Secret = priorSecret
	} else {
		model.Secret = types.StringNull()
	}

	return diags
}

func apiKeyDataSourceItemFromAPI(ctx context.Context, key cycle.ApiKey) (apiKeyDataSourceItemModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var ips []string
	if key.Ips != nil {
		ips = *key.Ips
	}
	list, d := types.ListValueFrom(ctx, types.StringType, stringSliceOrEmpty(ips))
	diags.Append(d...)
	if diags.HasError() {
		return apiKeyDataSourceItemModel{}, diags
	}

	return apiKeyDataSourceItemModel{
		ID:     types.StringValue(key.Id),
		Name:   types.StringValue(key.Name),
		RoleID: types.StringValue(key.RoleId),
		IPs:    list,
		HubID:  types.StringValue(key.HubId),
		State:  types.StringValue(string(key.State.Current)),
	}, diags
}
