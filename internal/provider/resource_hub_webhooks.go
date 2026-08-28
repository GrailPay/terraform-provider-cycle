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
	RegisterResource(NewHubWebhooksResource)
}

var (
	_ resource.Resource                = (*hubWebhooksResource)(nil)
	_ resource.ResourceWithConfigure   = (*hubWebhooksResource)(nil)
	_ resource.ResourceWithImportState = (*hubWebhooksResource)(nil)
)

// NewHubWebhooksResource returns the cycle_hub_webhooks resource.
func NewHubWebhooksResource() resource.Resource {
	return &hubWebhooksResource{}
}

type hubWebhooksResource struct {
	client *CycleClient
}

type hubWebhooksResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ServerDeployed types.String `tfsdk:"server_deployed"`
	ServerDeleted  types.String `tfsdk:"server_deleted"`
}

func (r *hubWebhooksResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub_webhooks"
}

func (r *hubWebhooksResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages hub-level webhooks on the current Cycle hub (`PATCH /v1/hubs/current`). This is a singleton — Terraform state is the source of truth, and create applies the configured URLs rather than creating a separate API resource. Destroy clears both webhook URLs.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the current hub.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"server_deployed": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Webhook URL invoked whenever a server is deployed to this hub. The payload is a `Server` object. Omit or set to `\"\"` to unset.",
			},
			"server_deleted": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Webhook URL invoked whenever a server in this hub is deleted. The payload is a `Server` object. Omit or set to `\"\"` to unset.",
			},
		},
	}
}

func (r *hubWebhooksResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *hubWebhooksResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan hubWebhooksResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hub, ok := r.updateHubWebhooks(ctx, plan, &resp.Diagnostics, "creating hub webhooks")
	if !ok {
		return
	}
	applyHubWebhooks(&plan, hub)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hubWebhooksResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state hubWebhooksResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hub, status, envelope, err := r.getHub(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading hub webhooks", err.Error())
		return
	}
	if status == http.StatusNotFound || hub == nil && envelope == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if hub == nil {
		addAPIError(&resp.Diagnostics, "reading hub webhooks", status, envelope)
		return
	}

	applyHubWebhooks(&state, *hub)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *hubWebhooksResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		return
	}

	var plan hubWebhooksResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hub, ok := r.updateHubWebhooks(ctx, plan, &resp.Diagnostics, "updating hub webhooks")
	if !ok {
		return
	}
	applyHubWebhooks(&plan, hub)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hubWebhooksResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}

	empty := ""
	body := cycle.UpdateHubJSONRequestBody{
		Webhooks: &cycle.HubWebhooks{
			ServerDeployed: &empty,
			ServerDeleted:  &empty,
		},
	}

	apiResp, err := r.client.Client.UpdateHubWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting hub webhooks", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "deleting hub webhooks", apiResp.StatusCode(), apiResp.JSONDefault)
	}
}

func (r *hubWebhooksResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if id == "current" {
		if r.client == nil {
			resp.Diagnostics.AddError(
				"Provider Not Configured",
				"Cannot import cycle_hub_webhooks with \"current\" because the provider is not configured.",
			)
			return
		}
		hub, status, envelope, err := r.getHub(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error reading hub during import", err.Error())
			return
		}
		if hub == nil {
			addAPIError(&resp.Diagnostics, "reading hub during import", status, envelope)
			return
		}
		id = hub.Id
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *hubWebhooksResource) getHub(ctx context.Context) (*cycle.Hub, int, *cycle.ErrorEnvelope, error) {
	apiResp, err := r.client.Client.GetHubWithResponse(ctx, &cycle.GetHubParams{})
	if err != nil {
		return nil, 0, nil, err
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return nil, apiResp.StatusCode(), nil, nil
	}
	if apiResp.JSON200 == nil || apiResp.JSON200.Data == nil {
		return nil, apiResp.StatusCode(), apiResp.JSONDefault, nil
	}
	return apiResp.JSON200.Data, apiResp.StatusCode(), nil, nil
}

func (r *hubWebhooksResource) updateHubWebhooks(ctx context.Context, model hubWebhooksResourceModel, diags *diag.Diagnostics, action string) (cycle.Hub, bool) {
	body := cycle.UpdateHubJSONRequestBody{
		Webhooks: hubWebhooksFromModel(model),
	}

	apiResp, err := r.client.Client.UpdateHubWithResponse(ctx, body)
	if err != nil {
		diags.AddError("Error "+action, err.Error())
		return cycle.Hub{}, false
	}
	if apiResp.JSON200 == nil {
		addAPIError(diags, action, apiResp.StatusCode(), apiResp.JSONDefault)
		return cycle.Hub{}, false
	}
	return apiResp.JSON200.Data, true
}

func hubWebhooksFromModel(model hubWebhooksResourceModel) *cycle.HubWebhooks {
	deployed := ""
	if !model.ServerDeployed.IsNull() && !model.ServerDeployed.IsUnknown() {
		deployed = model.ServerDeployed.ValueString()
	}
	deleted := ""
	if !model.ServerDeleted.IsNull() && !model.ServerDeleted.IsUnknown() {
		deleted = model.ServerDeleted.ValueString()
	}
	return &cycle.HubWebhooks{
		ServerDeployed: &deployed,
		ServerDeleted:  &deleted,
	}
}

func applyHubWebhooks(model *hubWebhooksResourceModel, hub cycle.Hub) {
	model.ID = types.StringValue(hub.Id)
	model.ServerDeployed = optionalWebhookURL(hub.Webhooks.ServerDeployed)
	model.ServerDeleted = optionalWebhookURL(hub.Webhooks.ServerDeleted)
}

func optionalWebhookURL(v *string) types.String {
	if v == nil || *v == "" {
		return types.StringNull()
	}
	return types.StringValue(*v)
}
