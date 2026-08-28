package provider

import (
	"context"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewHubRoleResource)
}

var (
	_ resource.Resource                = (*hubRoleResource)(nil)
	_ resource.ResourceWithConfigure   = (*hubRoleResource)(nil)
	_ resource.ResourceWithImportState = (*hubRoleResource)(nil)
)

// NewHubRoleResource returns the cycle_hub_role resource.
func NewHubRoleResource() resource.Resource {
	return &hubRoleResource{}
}

type hubRoleResource struct {
	client *CycleClient
}

type hubRoleResourceModel struct {
	ID           types.String              `tfsdk:"id"`
	Name         types.String              `tfsdk:"name"`
	Identifier   types.String              `tfsdk:"identifier"`
	Rank         types.Int64               `tfsdk:"rank"`
	Root         types.Bool                `tfsdk:"root"`
	Capabilities *hubRoleCapabilitiesModel `tfsdk:"capabilities"`
}

// hubRoleCapabilitiesModel mirrors the API's capabilities object on a Role:
// either the "all" flag, or a specific set of capability strings.
type hubRoleCapabilitiesModel struct {
	All      types.Bool `tfsdk:"all"`
	Specific types.Set  `tfsdk:"specific"`
}

func hubRoleCapabilitiesSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required:            true,
		MarkdownDescription: "The platform-level capabilities assigned to this role.",
		Attributes: map[string]schema.Attribute{
			"all": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "If `true`, the role has every platform capability and `specific` is ignored. Defaults to `false`.",
			},
			"specific": schema.SetAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})),
				MarkdownDescription: "The set of specific capability identifiers granted to this role (e.g. `environments-view`, `containers-manage`). Ignored when `all` is `true`.",
			},
		},
	}
}

func (r *hubRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub_role"
}

func (r *hubRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom role on the current Cycle hub. Roles are custom combinations of platform-level capabilities used for role-based access control (`/v1/hubs/current/roles`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the role.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A human-readable name for the role. If omitted, Cycle derives one from the identifier.",
			},
			"identifier": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A human-readable slug identifier for the role, e.g. `deploy-bot`.",
			},
			"rank": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "An integer between 0 and 10 indicating the role hierarchy. An account can only edit roles with a rank lower than its own; the built-in `owner` role is rank 10.",
				Validators: []validator.Int64{
					int64validator.Between(0, 10),
				},
			},
			"root": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this role is the root role with full moderation control over all other roles. Only the built-in owner role is root; always `false` for custom roles.",
			},
			"capabilities": hubRoleCapabilitiesSchema(),
		},
	}
}

func (r *hubRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

// roleCapabilitiesBody is the anonymous capabilities struct shared by the
// generated create and update role request bodies.
type roleCapabilitiesBody = struct {
	All      bool               `json:"all"`
	Specific []cycle.Capability `json:"specific"`
}

// capabilitiesFromModel converts the Terraform capabilities model into the
// request body shape used by both create and update.
func capabilitiesFromModel(ctx context.Context, m *hubRoleCapabilitiesModel, diags *diag.Diagnostics) *roleCapabilitiesBody {
	body := &roleCapabilitiesBody{
		All:      m.All.ValueBool(),
		Specific: []cycle.Capability{},
	}
	if !m.Specific.IsNull() && !m.Specific.IsUnknown() {
		var raw []string
		diags.Append(m.Specific.ElementsAs(ctx, &raw, false)...)
		if diags.HasError() {
			return nil
		}
		for _, s := range raw {
			body.Specific = append(body.Specific, cycle.Capability(s))
		}
	}
	return body
}

// setRoleState maps an API Role onto the Terraform model.
func setRoleState(ctx context.Context, role *cycle.Role, m *hubRoleResourceModel, diags *diag.Diagnostics) {
	m.ID = types.StringValue(role.Id)
	m.Identifier = types.StringValue(role.Identifier)
	m.Rank = types.Int64Value(int64(role.Rank))
	m.Root = types.BoolValue(role.Root)
	if role.Name != nil {
		m.Name = types.StringValue(*role.Name)
	} else {
		m.Name = types.StringNull()
	}

	specific := make([]string, 0, len(role.Capabilities.Specific))
	for _, c := range role.Capabilities.Specific {
		specific = append(specific, string(c))
	}
	specificSet, d := types.SetValueFrom(ctx, types.StringType, specific)
	diags.Append(d...)
	if diags.HasError() {
		return
	}
	m.Capabilities = &hubRoleCapabilitiesModel{
		All:      types.BoolValue(role.Capabilities.All),
		Specific: specificSet,
	}
}

func (r *hubRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan hubRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	caps := capabilitiesFromModel(ctx, plan.Capabilities, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.CreateRoleJSONRequestBody{
		Identifier:   plan.Identifier.ValueString(),
		Rank:         int(plan.Rank.ValueInt64()),
		Capabilities: caps,
	}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		body.Name = plan.Name.ValueStringPointer()
	}

	apiResp, err := r.client.Client.CreateRoleWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating hub role", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating hub role", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	role := apiResp.JSON201.Data
	setRoleState(ctx, &role, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hubRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state hubRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.GetRoleWithResponse(ctx, state.ID.ValueString(), &cycle.GetRoleParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading hub role", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil || apiResp.JSON200.Data == nil {
		addAPIError(&resp.Diagnostics, "reading hub role", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	role := apiResp.JSON200.Data
	if role.State.Current == "deleted" {
		resp.State.RemoveResource(ctx)
		return
	}

	setRoleState(ctx, role, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *hubRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		return
	}

	var plan hubRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state hubRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	caps := capabilitiesFromModel(ctx, plan.Capabilities, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.UpdateRoleJSONRequestBody{
		Identifier:   plan.Identifier.ValueString(),
		Rank:         int(plan.Rank.ValueInt64()),
		Capabilities: caps,
	}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		body.Name = plan.Name.ValueStringPointer()
	}

	apiResp, err := r.client.Client.UpdateRoleWithResponse(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating hub role", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating hub role", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	role := apiResp.JSON200.Data
	setRoleState(ctx, &role, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hubRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}

	var state hubRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.DeleteRoleWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting hub role", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return
	}
	if apiResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting hub role", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	if job := apiResp.JSON202.Data.Job; job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error deleting hub role", err.Error())
		}
	}
}

func (r *hubRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
