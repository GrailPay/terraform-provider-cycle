package provider

import (
	"context"
	"encoding/json"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewNetworkResource)
}

var (
	_ resource.Resource                = (*networkResource)(nil)
	_ resource.ResourceWithConfigure   = (*networkResource)(nil)
	_ resource.ResourceWithImportState = (*networkResource)(nil)
)

// NewNetworkResource returns the cycle_network resource.
func NewNetworkResource() resource.Resource {
	return &networkResource{}
}

type networkResource struct {
	client *CycleClient
}

type networkModel struct {
	ID             types.String         `tfsdk:"id"`
	Name           types.String         `tfsdk:"name"`
	Identifier     types.String         `tfsdk:"identifier"`
	Cluster        types.String         `tfsdk:"cluster"`
	EnvironmentIDs types.Set            `tfsdk:"environment_ids"`
	ACL            jsontypes.Normalized `tfsdk:"acl"`
	HubID          types.String         `tfsdk:"hub_id"`
	State          types.String         `tfsdk:"state"`
}

func (r *networkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *networkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyIDs := types.SetValueMust(types.StringType, []attr.Value{})

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle SDN network (`/v1/sdn/networks`). `cluster` and `identifier` cannot " +
			"be changed in place. `name` updates via `PATCH`; `acl` updates via the access endpoint; " +
			"`environment_ids` updates via a `reconfigure` job.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the SDN network.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A user-defined name for the network.",
			},
			"identifier": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A network identifier used to construct HTTP calls that specifically use this network. Changing this forces a new network to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cluster": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The identifier of the cluster whose environments may join this network. Changing this forces a new network to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_ids": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(emptyIDs),
				MarkdownDescription: "Environment IDs attached to this network. Defaults to an empty set. Changing membership issues a `reconfigure` job.",
			},
			"acl": schema.StringAttribute{
				Optional:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Network ACL as a JSON object (`{ \"roles\": { \"<role-id>\": { \"view\": bool, \"modify\": bool, \"manage\": bool } } }`).",
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this network belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the network (e.g. `live`).",
			},
		},
	}
}

func (r *networkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *networkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		return
	}

	var plan networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envIDs, diags := networkEnvironmentIDsFromValue(ctx, plan.EnvironmentIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	acl, diags := networkACLFromValue(plan.ACL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.CreateNetworkJSONRequestBody{
		Name:         plan.Name.ValueString(),
		Identifier:   plan.Identifier.ValueString(),
		Cluster:      plan.Cluster.ValueString(),
		Environments: envIDs,
		Acl:          acl,
	}

	createResp, err := r.client.Client.CreateNetworkWithResponse(ctx, &cycle.CreateNetworkParams{}, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating network", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating network", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(networkModelFromAPI(ctx, &plan, createResp.JSON201.Data, networkManagesACL(plan.ACL))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		return
	}

	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	net, status, envelope, err := getNetwork(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading network", err.Error())
		return
	}
	if status == http.StatusNotFound || networkGone(net) {
		resp.State.RemoveResource(ctx)
		return
	}
	if net == nil {
		addAPIError(&resp.Diagnostics, "reading network", status, envelope)
		return
	}

	resp.Diagnostics.Append(networkModelFromAPI(ctx, &state, *net, networkManagesACL(state.ACL))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		return
	}

	var plan networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateResp, err := r.client.Client.UpdateNetworkWithResponse(ctx, state.ID.ValueString(), &cycle.UpdateNetworkParams{}, cycle.UpdateNetworkJSONRequestBody{
			Name: &name,
		})
		if err != nil {
			resp.Diagnostics.AddError("Error updating network", err.Error())
			return
		}
		if updateResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "updating network", updateResp.StatusCode(), updateResp.JSONDefault)
			return
		}
	}

	if !plan.ACL.Equal(state.ACL) {
		acl, diags := networkACLFromValue(plan.ACL)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		accessResp, err := r.client.Client.UpdateNetworkAccessWithResponse(ctx, state.ID.ValueString(), &cycle.UpdateNetworkAccessParams{}, cycle.UpdateNetworkAccessJSONRequestBody{
			Acl: acl,
		})
		if err != nil {
			resp.Diagnostics.AddError("Error updating network access", err.Error())
			return
		}
		if accessResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "updating network access", accessResp.StatusCode(), accessResp.JSONDefault)
			return
		}
	}

	if !plan.EnvironmentIDs.Equal(state.EnvironmentIDs) {
		envIDs, diags := networkEnvironmentIDsFromValue(ctx, plan.EnvironmentIDs)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !r.reconfigureNetworkEnvironments(ctx, state.ID.ValueString(), envIDs, &resp.Diagnostics) {
			return
		}
	}

	net, status, envelope, err := getNetwork(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading network", err.Error())
		return
	}
	if net == nil {
		addAPIError(&resp.Diagnostics, "reading network", status, envelope)
		return
	}

	resp.Diagnostics.Append(networkModelFromAPI(ctx, &plan, *net, networkManagesACL(plan.ACL))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}

	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteNetworkWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting network", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting network", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}
	if job := deleteResp.JSON202.Data.Job; job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for network deletion", err.Error())
		}
	}
}

func (r *networkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *networkResource) reconfigureNetworkEnvironments(ctx context.Context, networkID string, environmentIDs []string, diags *diag.Diagnostics) bool {
	action := cycle.ReconfigureSdnNetworkAction{
		Action: cycle.ReconfigureSdnNetworkActionActionReconfigure,
	}
	action.Contents.EnvironmentIds = &environmentIDs

	var task cycle.SdnNetworkTask
	if err := task.FromReconfigureSdnNetworkAction(action); err != nil {
		diags.AddError("Error building network reconfigure task", err.Error())
		return false
	}

	apiResp, err := r.client.Client.CreateNetworkJobWithResponse(ctx, networkID, task)
	if err != nil {
		diags.AddError("Error reconfiguring network environments", err.Error())
		return false
	}
	if apiResp.JSON202 == nil {
		addAPIError(diags, "reconfiguring network environments", apiResp.StatusCode(), apiResp.JSONDefault)
		return false
	}
	if job := apiResp.JSON202.Data.Job; job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
			diags.AddError("Error waiting for network reconfigure", err.Error())
			return false
		}
	}
	return true
}

func getNetwork(ctx context.Context, client *CycleClient, networkID string) (*cycle.Network, int, *cycle.ErrorEnvelope, error) {
	apiResp, err := client.Client.GetNetworkWithResponse(ctx, networkID, &cycle.GetNetworkParams{})
	if err != nil {
		return nil, 0, nil, err
	}
	if apiResp.JSON200 == nil {
		return nil, apiResp.StatusCode(), apiResp.JSONDefault, nil
	}
	return &apiResp.JSON200.Data, apiResp.StatusCode(), nil, nil
}

func networkGone(net *cycle.Network) bool {
	if net == nil {
		return false
	}
	switch net.State.Current {
	case cycle.NetworkStateCurrentDeleted, cycle.NetworkStateCurrentDeleting:
		return true
	}
	return false
}

func networkManagesACL(acl jsontypes.Normalized) bool {
	return !acl.IsNull() && !acl.IsUnknown()
}

func networkACLFromValue(value jsontypes.Normalized) (*cycle.ACL, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var acl cycle.ACL
	diags := value.Unmarshal(&acl)
	if diags.HasError() {
		return nil, diags
	}
	return &acl, nil
}

func networkACLToValue(acl *cycle.ACL) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if acl == nil {
		return jsontypes.NewNormalizedNull(), diags
	}

	b, err := json.Marshal(acl)
	if err != nil {
		diags.AddError("Error encoding network ACL", err.Error())
		return jsontypes.NewNormalizedNull(), diags
	}
	if len(b) == 0 || string(b) == "null" {
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

func networkEnvironmentIDsFromValue(ctx context.Context, value types.Set) ([]string, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return []string{}, nil
	}

	var ids []string
	diags := value.ElementsAs(ctx, &ids, false)
	if ids == nil {
		ids = []string{}
	}
	return ids, diags
}

func networkEnvironmentIDsFromAPI(ctx context.Context, environments *[]struct {
	Added cycle.DateTime `json:"added"`
	Id    cycle.ID       `json:"id"`
}) (types.Set, diag.Diagnostics) {
	ids := make([]string, 0)
	if environments != nil {
		for _, env := range *environments {
			ids = append(ids, env.Id)
		}
	}
	return types.SetValueFrom(ctx, types.StringType, ids)
}

func networkModelFromAPI(ctx context.Context, model *networkModel, net cycle.Network, manageACL bool) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(net.Id)
	model.Name = types.StringValue(net.Name)
	model.Identifier = types.StringValue(net.Identifier)
	model.Cluster = types.StringValue(net.Cluster)
	model.HubID = types.StringValue(net.HubId)
	model.State = types.StringValue(string(net.State.Current))

	envIDs, d := networkEnvironmentIDsFromAPI(ctx, net.Environments)
	diags.Append(d...)
	model.EnvironmentIDs = envIDs

	if manageACL {
		acl, d := networkACLToValue(net.Acl)
		diags.Append(d...)
		model.ACL = acl
	}

	return diags
}
