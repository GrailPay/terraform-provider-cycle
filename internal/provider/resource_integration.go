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
	RegisterResource(NewIntegrationResource)
}

var (
	_ resource.Resource                = (*integrationResource)(nil)
	_ resource.ResourceWithConfigure   = (*integrationResource)(nil)
	_ resource.ResourceWithImportState = (*integrationResource)(nil)
)

// NewIntegrationResource returns the cycle_integration resource.
func NewIntegrationResource() resource.Resource {
	return &integrationResource{}
}

type integrationResource struct {
	client *CycleClient
}

type integrationModel struct {
	ID         types.String          `tfsdk:"id"`
	Name       types.String          `tfsdk:"name"`
	Identifier types.String          `tfsdk:"identifier"`
	Vendor     types.String          `tfsdk:"vendor"`
	Auth       *integrationAuthModel `tfsdk:"auth"`
	Extra      types.Map             `tfsdk:"extra"`
	HubID      types.String          `tfsdk:"hub_id"`
	State      types.String          `tfsdk:"state"`
}

type integrationAuthModel struct {
	APIKey         types.String `tfsdk:"api_key"`
	Base64Config   types.String `tfsdk:"base64_config"`
	ClientID       types.String `tfsdk:"client_id"`
	KeyID          types.String `tfsdk:"key_id"`
	Namespace      types.String `tfsdk:"namespace"`
	Region         types.String `tfsdk:"region"`
	Secret         types.String `tfsdk:"secret"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
}

func (r *integrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (r *integrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle hub integration (`/v1/integrations`). Integrations connect a vendor " +
			"(billing, image builders, infrastructure providers, object storage, or TLS certificate generation) " +
			"to the hub. `vendor` is immutable. Authentication and `extra` values are write-sensitive: later " +
			"reads preserve state when the API omits secrets.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A user-defined name for the integration.",
			},
			"identifier": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A human-readable slugged identifier for the integration. Automatically generated from the name if not provided.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vendor": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The vendor this integration is associated with (e.g. `aws`). Changing this forces a new integration to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"auth": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Vendor-specific authentication credentials. These values are write-sensitive; later reads preserve state when the API omits them.",
				Attributes:          integrationAuthAttributes(),
			},
			"extra": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Additional vendor-specific key-value pairs. Write-sensitive; later reads preserve state when the API omits this map.",
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this integration belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the integration (e.g. `live`).",
			},
		},
	}
}

func (r *integrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *integrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan integrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.CreateIntegrationJSONRequestBody{
		Name:       plan.Name.ValueString(),
		Vendor:     plan.Vendor.ValueString(),
		Identifier: plan.Identifier.ValueString(),
	}

	if auth := integrationAuthToAPI(plan.Auth); auth != nil {
		body.Auth = &struct {
			ApiKey         *string `json:"api_key,omitempty"`
			Base64Config   *string `json:"base64_config,omitempty"`
			ClientId       *string `json:"client_id,omitempty"`
			KeyId          *string `json:"key_id,omitempty"`
			Namespace      *string `json:"namespace,omitempty"`
			Region         *string `json:"region,omitempty"`
			Secret         *string `json:"secret,omitempty"`
			SubscriptionId *string `json:"subscription_id,omitempty"`
		}{
			ApiKey:         auth.ApiKey,
			Base64Config:   auth.Base64Config,
			ClientId:       auth.ClientId,
			KeyId:          auth.KeyId,
			Namespace:      auth.Namespace,
			Region:         auth.Region,
			Secret:         auth.Secret,
			SubscriptionId: auth.SubscriptionId,
		}
	}

	extra, d := integrationExtraToAPI(ctx, plan.Extra)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Extra = extra

	createResp, err := r.client.Client.CreateIntegrationWithResponse(ctx, &cycle.CreateIntegrationParams{}, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating integration", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating integration", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(integrationModelFromAPI(ctx, &plan, createResp.JSON201.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *integrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state integrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetIntegrationWithResponse(ctx, state.ID.ValueString(), &cycle.GetIntegrationParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading integration", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading integration", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	integ := getResp.JSON200.Data
	if integ.State.Current == cycle.IntegrationStateCurrentDeleted || integ.State.Current == cycle.IntegrationStateCurrentDeleting {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(integrationModelFromAPI(ctx, &state, integ)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *integrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan integrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state integrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	body := cycle.UpdateIntegrationJSONRequestBody{
		Name: &name,
		Auth: integrationAuthToAPI(plan.Auth),
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}

	extra, d := integrationExtraToAPI(ctx, plan.Extra)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Extra = extra

	updateResp, err := r.client.Client.UpdateIntegrationWithResponse(ctx, state.ID.ValueString(), &cycle.UpdateIntegrationParams{}, body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating integration", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating integration", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	// Keep write-sensitive values from state unless the update response includes replacements.
	if plan.Auth == nil {
		plan.Auth = state.Auth
	}
	if plan.Extra.IsNull() || plan.Extra.IsUnknown() {
		plan.Extra = state.Extra
	}

	resp.Diagnostics.Append(integrationModelFromAPI(ctx, &plan, updateResp.JSON200.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *integrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state integrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteIntegrationWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting integration", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting integration", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}
	if job := deleteResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for integration deletion", err.Error())
		}
	}
}

func (r *integrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func integrationAuthAttributes() map[string]schema.Attribute {
	field := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Optional:            true,
			Sensitive:           true,
			MarkdownDescription: desc,
		}
	}

	return map[string]schema.Attribute{
		"api_key":         field("API key for accessing the integration."),
		"base64_config":   field("Base64-encoded configuration for the integration."),
		"client_id":       field("Client ID for the integration."),
		"key_id":          field("Key ID for accessing the integration."),
		"namespace":       field("The namespace associated with the integration."),
		"region":          field("The region associated with the integration."),
		"secret":          field("Secret for accessing the integration."),
		"subscription_id": field("Subscription ID for the integration."),
	}
}

func integrationAuthToAPI(auth *integrationAuthModel) *cycle.IntegrationAuth {
	if auth == nil {
		return nil
	}

	out := &cycle.IntegrationAuth{}
	anySet := false
	if v := optionalStringPtr(auth.APIKey); v != nil {
		out.ApiKey = v
		anySet = true
	}
	if v := optionalStringPtr(auth.Base64Config); v != nil {
		out.Base64Config = v
		anySet = true
	}
	if v := optionalStringPtr(auth.ClientID); v != nil {
		out.ClientId = v
		anySet = true
	}
	if v := optionalStringPtr(auth.KeyID); v != nil {
		out.KeyId = v
		anySet = true
	}
	if v := optionalStringPtr(auth.Namespace); v != nil {
		out.Namespace = v
		anySet = true
	}
	if v := optionalStringPtr(auth.Region); v != nil {
		out.Region = v
		anySet = true
	}
	if v := optionalStringPtr(auth.Secret); v != nil {
		out.Secret = v
		anySet = true
	}
	if v := optionalStringPtr(auth.SubscriptionID); v != nil {
		out.SubscriptionId = v
		anySet = true
	}
	if !anySet {
		return nil
	}
	return out
}

func optionalStringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil
	}
	s := v.ValueString()
	return &s
}

func integrationExtraToAPI(ctx context.Context, extra types.Map) (*map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if extra.IsNull() || extra.IsUnknown() {
		return nil, diags
	}
	var m map[string]string
	diags.Append(extra.ElementsAs(ctx, &m, false)...)
	if diags.HasError() {
		return nil, diags
	}
	return &m, diags
}

// integrationModelFromAPI maps a Cycle Integration onto the shared Terraform
// model used by the resource and both data sources. Auth and extra are
// write-sensitive: empty/omitted API values keep whatever was already in model.
func integrationModelFromAPI(ctx context.Context, model *integrationModel, integ cycle.Integration) diag.Diagnostics {
	var diags diag.Diagnostics

	priorAuth := model.Auth
	priorExtra := model.Extra

	model.ID = types.StringValue(integ.Id)
	model.Name = types.StringValue(integ.Name)
	model.Identifier = types.StringValue(integ.Identifier)
	model.Vendor = types.StringValue(integ.Vendor)
	model.HubID = types.StringValue(integ.HubId)
	model.State = types.StringValue(string(integ.State.Current))
	model.Auth = integrationAuthFromAPI(priorAuth, integ.Auth)

	if integ.Extra != nil {
		extra, d := types.MapValueFrom(ctx, types.StringType, *integ.Extra)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		model.Extra = extra
	} else if !priorExtra.IsNull() && !priorExtra.IsUnknown() {
		model.Extra = priorExtra
	} else {
		model.Extra = types.MapNull(types.StringType)
	}

	return diags
}

func integrationAuthFromAPI(prior *integrationAuthModel, auth *cycle.IntegrationAuth) *integrationAuthModel {
	if auth == nil {
		return prior
	}

	out := &integrationAuthModel{
		APIKey:         preserveSensitiveString(priorAuthField(prior, func(p *integrationAuthModel) types.String { return p.APIKey }), auth.ApiKey),
		Base64Config:   preserveSensitiveString(priorAuthField(prior, func(p *integrationAuthModel) types.String { return p.Base64Config }), auth.Base64Config),
		ClientID:       preserveSensitiveString(priorAuthField(prior, func(p *integrationAuthModel) types.String { return p.ClientID }), auth.ClientId),
		KeyID:          preserveSensitiveString(priorAuthField(prior, func(p *integrationAuthModel) types.String { return p.KeyID }), auth.KeyId),
		Namespace:      preserveSensitiveString(priorAuthField(prior, func(p *integrationAuthModel) types.String { return p.Namespace }), auth.Namespace),
		Region:         preserveSensitiveString(priorAuthField(prior, func(p *integrationAuthModel) types.String { return p.Region }), auth.Region),
		Secret:         preserveSensitiveString(priorAuthField(prior, func(p *integrationAuthModel) types.String { return p.Secret }), auth.Secret),
		SubscriptionID: preserveSensitiveString(priorAuthField(prior, func(p *integrationAuthModel) types.String { return p.SubscriptionID }), auth.SubscriptionId),
	}

	if out.APIKey.IsNull() && out.Base64Config.IsNull() && out.ClientID.IsNull() && out.KeyID.IsNull() &&
		out.Namespace.IsNull() && out.Region.IsNull() && out.Secret.IsNull() && out.SubscriptionID.IsNull() {
		return prior
	}
	return out
}

func priorAuthField(prior *integrationAuthModel, get func(*integrationAuthModel) types.String) types.String {
	if prior == nil {
		return types.StringNull()
	}
	return get(prior)
}

func preserveSensitiveString(prior types.String, api *string) types.String {
	if api != nil && *api != "" {
		return types.StringValue(*api)
	}
	if !prior.IsNull() && !prior.IsUnknown() && prior.ValueString() != "" {
		return prior
	}
	return types.StringNull()
}
