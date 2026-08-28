package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewScopedVariableResource)
}

var (
	_ resource.Resource                = (*scopedVariableResource)(nil)
	_ resource.ResourceWithConfigure   = (*scopedVariableResource)(nil)
	_ resource.ResourceWithImportState = (*scopedVariableResource)(nil)
)

// NewScopedVariableResource returns the cycle_scoped_variable resource.
func NewScopedVariableResource() resource.Resource {
	return &scopedVariableResource{}
}

type scopedVariableResource struct {
	client *CycleClient
}

type scopedVariableResourceModel struct {
	ID            types.String               `tfsdk:"id"`
	EnvironmentID types.String               `tfsdk:"environment_id"`
	Identifier    types.String               `tfsdk:"identifier"`
	Value         types.String               `tfsdk:"value"`
	Scope         *scopedVariableScopeModel  `tfsdk:"scope"`
	Access        *scopedVariableAccessModel `tfsdk:"access"`
	HubID         types.String               `tfsdk:"hub_id"`
}

type scopedVariableScopeModel struct {
	Global               types.Bool `tfsdk:"global"`
	ContainerIDs         types.List `tfsdk:"container_ids"`
	ContainerIdentifiers types.List `tfsdk:"container_identifiers"`
}

type scopedVariableAccessModel struct {
	EnvVariable *scopedVariableEnvVariableModel `tfsdk:"env_variable"`
	InternalAPI *scopedVariableInternalAPIModel `tfsdk:"internal_api"`
	File        *scopedVariableFileModel        `tfsdk:"file"`
}

type scopedVariableEnvVariableModel struct {
	Key types.String `tfsdk:"key"`
}

type scopedVariableInternalAPIModel struct {
	Duration types.String `tfsdk:"duration"`
}

type scopedVariableFileModel struct {
	Path   types.String `tfsdk:"path"`
	Decode types.Bool   `tfsdk:"decode"`
}

func (r *scopedVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scoped_variable"
}

func (r *scopedVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyStringList := types.ListValueMust(types.StringType, []attr.Value{})

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle scoped variable. Scoped variables dynamically allocate runtime values (environment variables, files, or internal API secrets) across containers in an environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the scoped variable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment this scoped variable belongs to. Changing this forces a new scoped variable to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"identifier": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The identifier for this scoped variable, similar to the key of an environment variable.",
			},
			"value": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "The raw value of the scoped variable.",
			},
			"scope": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Controls which containers in the environment the variable is assigned to. If omitted, the variable is assigned globally to all current and future containers in the environment.",
				Attributes: map[string]schema.Attribute{
					"global": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
						MarkdownDescription: "When `true`, the variable is assigned to all current and future containers in the environment. Defaults to `false` when the `scope` block is specified explicitly.",
					},
					"container_ids": schema.ListAttribute{
						ElementType:         types.StringType,
						Optional:            true,
						Computed:            true,
						Default:             listdefault.StaticValue(emptyStringList),
						MarkdownDescription: "A list of container IDs that have access to this scoped variable.",
					},
					"container_identifiers": schema.ListAttribute{
						ElementType:         types.StringType,
						Optional:            true,
						Computed:            true,
						Default:             listdefault.StaticValue(emptyStringList),
						MarkdownDescription: "A list of container identifiers that have access to this scoped variable.",
					},
				},
				Default: objectdefault.StaticValue(types.ObjectValueMust(
					map[string]attr.Type{
						"global":                types.BoolType,
						"container_ids":         types.ListType{ElemType: types.StringType},
						"container_identifiers": types.ListType{ElemType: types.StringType},
					},
					map[string]attr.Value{
						"global":                types.BoolValue(true),
						"container_ids":         emptyStringList,
						"container_identifiers": emptyStringList,
					},
				)),
			},
			"access": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Controls how the scoped variable is exposed to containers. At least one of `env_variable`, `internal_api`, or `file` should be set.",
				Attributes: map[string]schema.Attribute{
					"env_variable": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Expose the variable as an environment variable inside the container.",
						Attributes: map[string]schema.Attribute{
							"key": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The name of the environment variable set on the target container.",
							},
						},
					},
					"internal_api": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Expose the variable over Cycle's internal API.",
						Attributes: map[string]schema.Attribute{
							"duration": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "A duration string (e.g. `5m`) for which the internal API will serve the variable after the container runtime starts. If unset, the variable is served indefinitely.",
							},
						},
					},
					"file": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Expose the variable as a file mounted inside the container.",
						Attributes: map[string]schema.Attribute{
							"path": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "The path to mount the file to inside the container.",
							},
							"decode": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
								MarkdownDescription: "When `true`, Cycle interprets the value as a base64-encoded string and decodes it before passing it into the container. Defaults to `false`.",
							},
						},
					},
				},
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this scoped variable belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *scopedVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *scopedVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scopedVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scope := scopedVariableScopeToAPI(ctx, plan.Scope, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var source cycle.CreateScopedVariableJSONBody_Source
	if err := source.FromRawSource(scopedVariableRawSource(plan.Value.ValueString())); err != nil {
		resp.Diagnostics.AddError("Error building scoped variable source", err.Error())
		return
	}

	body := cycle.CreateScopedVariableJSONRequestBody{
		Identifier: plan.Identifier.ValueString(),
		Scope:      scope,
		Source:     source,
		Access:     scopedVariableAccessToAPI(plan.Access),
	}

	createResp, err := r.client.Client.CreateScopedVariableWithResponse(ctx, plan.EnvironmentID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating scoped variable", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating scoped variable", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	scopedVariableModelFromAPI(ctx, &plan, createResp.JSON201.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *scopedVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scopedVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetScopedVariableWithResponse(ctx, state.EnvironmentID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading scoped variable", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading scoped variable", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	scopedVariableModelFromAPI(ctx, &state, getResp.JSON200.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *scopedVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan scopedVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state scopedVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scope := scopedVariableScopeToAPI(ctx, plan.Scope, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var source cycle.UpdateScopedVariableJSONBody_Source
	if err := source.FromRawSource(scopedVariableRawSource(plan.Value.ValueString())); err != nil {
		resp.Diagnostics.AddError("Error building scoped variable source", err.Error())
		return
	}

	identifier := plan.Identifier.ValueString()
	body := cycle.UpdateScopedVariableJSONRequestBody{
		Identifier: &identifier,
		Scope:      &scope,
		Source:     &source,
		Access:     scopedVariableAccessToAPI(plan.Access),
	}

	updateResp, err := r.client.Client.UpdateScopedVariableWithResponse(ctx, state.EnvironmentID.ValueString(), state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating scoped variable", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating scoped variable", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	scopedVariableModelFromAPI(ctx, &plan, updateResp.JSON200.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *scopedVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scopedVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteScopedVariableWithResponse(ctx, state.EnvironmentID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting scoped variable", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting scoped variable", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}

	if job := deleteResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for scoped variable deletion", err.Error())
		}
	}
}

// ImportState imports a scoped variable using the composite ID
// "environment_id/scoped_variable_id", e.g.
//
//	terraform import cycle_scoped_variable.example 651e...9f2/651f...a01
func (r *scopedVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected an import ID in the form \"environment_id/scoped_variable_id\", got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// scopedVariableRawSource builds the raw (static value) source payload shared
// by create and update.
func scopedVariableRawSource(value string) cycle.RawSource {
	var raw cycle.RawSource
	raw.Type = cycle.RawSourceTypeRaw
	raw.Details.Value = value
	raw.Details.Blob = strings.Contains(value, "\n")
	return raw
}

func scopedVariableScopeToAPI(ctx context.Context, m *scopedVariableScopeModel, diags *diag.Diagnostics) cycle.ScopedVariableScope {
	var scope cycle.ScopedVariableScope
	scope.Containers.Ids = []string{}
	scope.Containers.Identifiers = []string{}
	if m == nil {
		scope.Containers.Global = true
		return scope
	}

	scope.Containers.Global = m.Global.ValueBool()
	if !m.ContainerIDs.IsNull() && !m.ContainerIDs.IsUnknown() {
		diags.Append(m.ContainerIDs.ElementsAs(ctx, &scope.Containers.Ids, false)...)
	}
	if !m.ContainerIdentifiers.IsNull() && !m.ContainerIdentifiers.IsUnknown() {
		diags.Append(m.ContainerIdentifiers.ElementsAs(ctx, &scope.Containers.Identifiers, false)...)
	}
	if scope.Containers.Ids == nil {
		scope.Containers.Ids = []string{}
	}
	if scope.Containers.Identifiers == nil {
		scope.Containers.Identifiers = []string{}
	}
	return scope
}

func scopedVariableAccessToAPI(m *scopedVariableAccessModel) *cycle.ScopedVariableAccess {
	if m == nil {
		return nil
	}

	access := &cycle.ScopedVariableAccess{}
	if m.EnvVariable != nil {
		access.EnvVariable = &struct {
			Key string `json:"key"`
		}{
			Key: m.EnvVariable.Key.ValueString(),
		}
	}
	if m.InternalAPI != nil {
		internalAPI := &struct {
			Duration *cycle.Duration `json:"duration,omitempty"`
		}{}
		if !m.InternalAPI.Duration.IsNull() && !m.InternalAPI.Duration.IsUnknown() {
			duration := m.InternalAPI.Duration.ValueString()
			internalAPI.Duration = &duration
		}
		access.InternalApi = internalAPI
	}
	if m.File != nil {
		file := &struct {
			Decode      bool    `json:"decode"`
			Gid         *int    `json:"gid,omitempty"`
			Path        *string `json:"path"`
			Permissions *string `json:"permissions,omitempty"`
			Uid         *int    `json:"uid,omitempty"`
		}{
			Decode: m.File.Decode.ValueBool(),
		}
		if !m.File.Path.IsNull() && !m.File.Path.IsUnknown() {
			p := m.File.Path.ValueString()
			file.Path = &p
		}
		access.File = file
	}
	return access
}

func scopedVariableModelFromAPI(ctx context.Context, model *scopedVariableResourceModel, sv cycle.ScopedVariable, diags *diag.Diagnostics) {
	model.ID = types.StringValue(sv.Id)
	model.EnvironmentID = types.StringValue(sv.EnvironmentId)
	model.Identifier = types.StringValue(sv.Identifier)
	model.HubID = types.StringValue(sv.HubId)

	// Only raw sources are managed by this resource; if the variable was
	// changed out-of-band to a URL source the previous value is kept in state
	// and the drift shows up on the next plan.
	if sv.Source != nil {
		if discriminator, err := sv.Source.Discriminator(); err == nil && discriminator == string(cycle.RawSourceTypeRaw) {
			if raw, err := sv.Source.AsRawSource(); err == nil {
				model.Value = types.StringValue(raw.Details.Value)
			}
		}
	}

	ids, d := types.ListValueFrom(ctx, types.StringType, stringSliceOrEmpty(sv.Scope.Containers.Ids))
	diags.Append(d...)
	identifiers, d := types.ListValueFrom(ctx, types.StringType, stringSliceOrEmpty(sv.Scope.Containers.Identifiers))
	diags.Append(d...)
	model.Scope = &scopedVariableScopeModel{
		Global:               types.BoolValue(sv.Scope.Containers.Global),
		ContainerIDs:         ids,
		ContainerIdentifiers: identifiers,
	}

	model.Access = scopedVariableAccessFromAPI(sv.Access)
}

func scopedVariableAccessFromAPI(access cycle.ScopedVariableAccess) *scopedVariableAccessModel {
	if access.EnvVariable == nil && access.InternalApi == nil && access.File == nil {
		return nil
	}

	m := &scopedVariableAccessModel{}
	if access.EnvVariable != nil {
		m.EnvVariable = &scopedVariableEnvVariableModel{
			Key: types.StringValue(access.EnvVariable.Key),
		}
	}
	if access.InternalApi != nil {
		internalAPI := &scopedVariableInternalAPIModel{
			Duration: types.StringNull(),
		}
		if access.InternalApi.Duration != nil {
			internalAPI.Duration = types.StringValue(*access.InternalApi.Duration)
		}
		m.InternalAPI = internalAPI
	}
	if access.File != nil {
		file := &scopedVariableFileModel{
			Path:   types.StringNull(),
			Decode: types.BoolValue(access.File.Decode),
		}
		if access.File.Path != nil {
			file.Path = types.StringValue(*access.File.Path)
		}
		m.File = file
	}
	return m
}

func stringSliceOrEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
