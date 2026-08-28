package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewServerResource)
}

var (
	_ resource.Resource                = (*serverResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverResource)(nil)
	_ resource.ResourceWithImportState = (*serverResource)(nil)
)

var serverResourceIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

// NewServerResource returns the cycle_server resource.
func NewServerResource() resource.Resource {
	return &serverResource{}
}

type serverResource struct {
	client *CycleClient
}

type serverResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Cluster             types.String `tfsdk:"cluster"`
	IntegrationID       types.String `tfsdk:"integration_id"`
	LocationID          types.String `tfsdk:"location_id"`
	ModelID             types.String `tfsdk:"model_id"`
	Zone                types.String `tfsdk:"zone"`
	AttachedStorageSize types.Int64  `tfsdk:"attached_storage_size"`
	EncryptStorage      types.Bool   `tfsdk:"encrypt_storage"`
	ReservationID       types.String `tfsdk:"reservation_id"`
	ForceDelete         types.Bool   `tfsdk:"force_delete"`
	Nickname            types.String `tfsdk:"nickname"`
	Tags                types.List   `tfsdk:"tags"`
	Hostname            types.String `tfsdk:"hostname"`
	State               types.String `tfsdk:"state"`
	HubID               types.String `tfsdk:"hub_id"`
	IPs                 types.List   `tfsdk:"ips"`
}

func (r *serverResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (r *serverResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyStringList := types.ListValueMust(types.StringType, []attr.Value{})

	resp.Schema = schema.Schema{
		MarkdownDescription: "Provisions a Cycle server into a cluster via `POST /v1/infrastructure/servers`. " +
			"Provisioning and deletion are asynchronous jobs; Terraform waits for each job to finish. " +
			"Changing cluster, provider integration, location, model, or advanced provision options forces a new server.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the server.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The identifier of the cluster to provision the server into. Changing this forces a new server to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"integration_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the provider integration used to provision this server. Changing this forces a new server to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The provider location ID to provision the server in. Changing this forces a new server to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"model_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The provider server model ID to provision. Changing this forces a new server to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"zone": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Availability zone for providers that support zone selection. Changing this forces a new server to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attached_storage_size": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Attached volume size in GB, for providers that support setting this dynamically. Changing this forces a new server to be created.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"encrypt_storage": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When `true`, encrypt storage at provision time for providers that support this setting. Changing this forces a new server to be created.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"reservation_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "A provider reservation ID to consume when provisioning a reserved server. Changing this forces a new server to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"force_delete": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, force-delete the server even if container instances are still running on it. Used only on destroy. Defaults to `false`.",
			},
			"nickname": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "A custom display name for the server. Does not affect the server hostname.",
			},
			"tags": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             listdefault.StaticValue(emptyStringList),
				MarkdownDescription: "Server constraint tags. Containers can target these tags when scheduling.",
			},
			"hostname": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The hostname assigned to the server by the platform.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the server (e.g. `provisioning`, `live`, `deleting`).",
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this server belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ips": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IP addresses assigned to the server during provisioning (`provider.init_ips`). Empty if the provider has not reported any yet.",
			},
		},
	}
}

func (r *serverResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *serverResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	item := struct {
		Advanced *struct {
			ProvisionOptions *struct {
				AttachedStorageSize *int    `json:"attached_storage_size,omitempty"`
				EncryptStorage      *bool   `json:"encrypt_storage,omitempty"`
				ReservationId       *string `json:"reservation_id,omitempty"`
			} `json:"provision_options,omitempty"`
			Zone *string `json:"zone,omitempty"`
		} `json:"advanced,omitempty"`
		IntegrationId cycle.ID `json:"integration_id"`
		LocationId    string   `json:"location_id"`
		ModelId       string   `json:"model_id"`
	}{
		IntegrationId: plan.IntegrationID.ValueString(),
		LocationId:    plan.LocationID.ValueString(),
		ModelId:       plan.ModelID.ValueString(),
	}

	if advanced := serverAdvancedFromPlan(plan); advanced != nil {
		item.Advanced = advanced
	}

	createResp, err := r.client.Client.CreateServersWithResponse(ctx, cycle.CreateServersJSONRequestBody{
		Cluster: plan.Cluster.ValueString(),
		Servers: []struct {
			Advanced *struct {
				ProvisionOptions *struct {
					AttachedStorageSize *int    `json:"attached_storage_size,omitempty"`
					EncryptStorage      *bool   `json:"encrypt_storage,omitempty"`
					ReservationId       *string `json:"reservation_id,omitempty"`
				} `json:"provision_options,omitempty"`
				Zone *string `json:"zone,omitempty"`
			} `json:"advanced,omitempty"`
			IntegrationId cycle.ID `json:"integration_id"`
			LocationId    string   `json:"location_id"`
			ModelId       string   `json:"model_id"`
		}{item},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating server", err.Error())
		return
	}
	if createResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "creating server", createResp.StatusCode(), createResp.JSONDefault)
		return
	}
	if createResp.JSON202.Data.Job == nil {
		resp.Diagnostics.AddError("Error creating server", "Cycle API returned a job descriptor without a job ID")
		return
	}

	job, err := waitForJob(ctx, r.client, createResp.JSON202.Data.Job.Id)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for server provision", err.Error())
		return
	}

	serverID, err := resolveProvisionedServerID(ctx, r.client, job, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving provisioned server ID", err.Error())
		return
	}

	getResp, err := r.client.Client.GetServerWithResponse(ctx, serverID, &cycle.GetServerParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading server after create", err.Error())
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading server after create", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	serverResourceModelFromAPI(ctx, &plan, getResp.JSON200.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.Nickname.IsNull() && !plan.Nickname.IsUnknown() || serverTagsConfigured(plan) {
		if err := applyServerUpdate(ctx, r.client, serverID, plan, &resp.Diagnostics); err != nil {
			resp.Diagnostics.AddError("Error applying server nickname/tags after create", err.Error())
			return
		}
		if resp.Diagnostics.HasError() {
			return
		}
		getResp, err = r.client.Client.GetServerWithResponse(ctx, serverID, &cycle.GetServerParams{})
		if err != nil {
			resp.Diagnostics.AddError("Error reading server after update", err.Error())
			return
		}
		if getResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "reading server after update", getResp.StatusCode(), getResp.JSONDefault)
			return
		}
		serverResourceModelFromAPI(ctx, &plan, getResp.JSON200.Data, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetServerWithResponse(ctx, state.ID.ValueString(), &cycle.GetServerParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading server", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading server", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	serverResourceModelFromAPI(ctx, &state, getResp.JSON200.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state serverResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := applyServerUpdate(ctx, r.client, state.ID.ValueString(), plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error updating server", err.Error())
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetServerWithResponse(ctx, state.ID.ValueString(), &cycle.GetServerParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading server after update", err.Error())
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading server after update", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	serverResourceModelFromAPI(ctx, &plan, getResp.JSON200.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	force := state.ForceDelete.ValueBool()
	body := cycle.DeleteServerJSONRequestBody{
		Options: &struct {
			Force *bool `json:"force,omitempty"`
		}{Force: &force},
	}

	deleteResp, err := r.client.Client.DeleteServerWithResponse(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting server", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}

	jobID, ok := serverDeleteJobID(deleteResp)
	if !ok {
		addAPIError(&resp.Diagnostics, "deleting server", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}
	if jobID != "" {
		if _, err := waitForJob(ctx, r.client, jobID); err != nil {
			resp.Diagnostics.AddError("Error waiting for server deletion", err.Error())
		}
	}
}

func (r *serverResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func serverAdvancedFromPlan(plan serverResourceModel) *struct {
	ProvisionOptions *struct {
		AttachedStorageSize *int    `json:"attached_storage_size,omitempty"`
		EncryptStorage      *bool   `json:"encrypt_storage,omitempty"`
		ReservationId       *string `json:"reservation_id,omitempty"`
	} `json:"provision_options,omitempty"`
	Zone *string `json:"zone,omitempty"`
} {
	var zone *string
	if !plan.Zone.IsNull() && !plan.Zone.IsUnknown() {
		z := plan.Zone.ValueString()
		zone = &z
	}

	var opts *struct {
		AttachedStorageSize *int    `json:"attached_storage_size,omitempty"`
		EncryptStorage      *bool   `json:"encrypt_storage,omitempty"`
		ReservationId       *string `json:"reservation_id,omitempty"`
	}
	if !plan.AttachedStorageSize.IsNull() && !plan.AttachedStorageSize.IsUnknown() ||
		!plan.EncryptStorage.IsNull() && !plan.EncryptStorage.IsUnknown() ||
		!plan.ReservationID.IsNull() && !plan.ReservationID.IsUnknown() {
		opts = &struct {
			AttachedStorageSize *int    `json:"attached_storage_size,omitempty"`
			EncryptStorage      *bool   `json:"encrypt_storage,omitempty"`
			ReservationId       *string `json:"reservation_id,omitempty"`
		}{}
		if !plan.AttachedStorageSize.IsNull() && !plan.AttachedStorageSize.IsUnknown() {
			size := int(plan.AttachedStorageSize.ValueInt64())
			opts.AttachedStorageSize = &size
		}
		if !plan.EncryptStorage.IsNull() && !plan.EncryptStorage.IsUnknown() {
			encrypt := plan.EncryptStorage.ValueBool()
			opts.EncryptStorage = &encrypt
		}
		if !plan.ReservationID.IsNull() && !plan.ReservationID.IsUnknown() {
			reservation := plan.ReservationID.ValueString()
			opts.ReservationId = &reservation
		}
	}

	if zone == nil && opts == nil {
		return nil
	}
	return &struct {
		ProvisionOptions *struct {
			AttachedStorageSize *int    `json:"attached_storage_size,omitempty"`
			EncryptStorage      *bool   `json:"encrypt_storage,omitempty"`
			ReservationId       *string `json:"reservation_id,omitempty"`
		} `json:"provision_options,omitempty"`
		Zone *string `json:"zone,omitempty"`
	}{ProvisionOptions: opts, Zone: zone}
}

func serverTagsConfigured(plan serverResourceModel) bool {
	return !plan.Tags.IsNull() && !plan.Tags.IsUnknown() && len(plan.Tags.Elements()) > 0
}

func applyServerUpdate(ctx context.Context, client *CycleClient, serverID string, plan serverResourceModel, diags *diag.Diagnostics) error {
	getResp, err := client.Client.GetServerWithResponse(ctx, serverID, &cycle.GetServerParams{})
	if err != nil {
		return err
	}
	if getResp.JSON200 == nil {
		addAPIError(diags, "reading server before update", getResp.StatusCode(), getResp.JSONDefault)
		return nil
	}
	current := getResp.JSON200.Data

	body := cycle.UpdateServerJSONRequestBody{}
	body.Constraints.Allow = &struct {
		Overcommit         bool `json:"overcommit"`
		OvercommitMultiple *int `json:"overcommit_multiple,omitempty"`
		Pool               bool `json:"pool"`
		Services           bool `json:"services"`
	}{
		Overcommit:         current.Constraints.Allow.Overcommit,
		OvercommitMultiple: current.Constraints.Allow.OvercommitMultiple,
		Pool:               current.Constraints.Allow.Pool,
		Services:           current.Constraints.Allow.Services,
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		var tags []string
		diags.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		if diags.HasError() {
			return nil
		}
		if tags == nil {
			tags = []string{}
		}
		body.Constraints.Tags = &tags
	}
	if !plan.Nickname.IsNull() && !plan.Nickname.IsUnknown() {
		nickname := plan.Nickname.ValueString()
		body.Nickname = &nickname
	}

	updateResp, err := client.Client.UpdateServerWithResponse(ctx, serverID, body)
	if err != nil {
		return err
	}
	if updateResp.JSON200 == nil {
		addAPIError(diags, "updating server", updateResp.StatusCode(), updateResp.JSONDefault)
	}
	return nil
}

func serverResourceModelFromAPI(ctx context.Context, model *serverResourceModel, srv cycle.Server, diags *diag.Diagnostics) {
	// Create-only optional inputs (zone, attached_storage_size, encrypt_storage,
	// reservation_id, force_delete) are left as-is so a refresh does not invent
	// values the configuration never set (which would force a replace).
	model.ID = types.StringValue(srv.Id)
	model.Cluster = types.StringValue(srv.Cluster)
	model.IntegrationID = types.StringValue(srv.Provider.IntegrationId)
	model.LocationID = types.StringValue(srv.LocationId)
	model.ModelID = types.StringValue(srv.ModelId)
	model.Hostname = types.StringValue(srv.Hostname)
	model.HubID = types.StringValue(srv.HubId)
	model.State = types.StringValue(string(srv.State.Current))
	if model.ForceDelete.IsNull() {
		model.ForceDelete = types.BoolValue(false)
	}

	if srv.Nickname != nil && *srv.Nickname != "" {
		model.Nickname = types.StringValue(*srv.Nickname)
	} else if model.Nickname.IsUnknown() {
		model.Nickname = types.StringNull()
	}

	tags := srv.Constraints.Tags
	if tags == nil {
		tags = []string{}
	}
	tagList, d := types.ListValueFrom(ctx, types.StringType, tags)
	diags.Append(d...)
	model.Tags = tagList

	ips := []string{}
	if srv.Provider.InitIps != nil {
		ips = *srv.Provider.InitIps
	}
	ipList, d := types.ListValueFrom(ctx, types.StringType, ips)
	diags.Append(d...)
	model.IPs = ipList
}

// resolveProvisionedServerID finds the server created by a provision job.
// It prefers a 24-character hex id in job.Tasks[i].Output, then falls back to
// listing servers in the cluster and picking the newest match on location/model.
func resolveProvisionedServerID(ctx context.Context, client *CycleClient, job *cycle.Job, plan serverResourceModel) (string, error) {
	if id := serverIDFromJob(job); id != "" {
		return id, nil
	}

	cluster := plan.Cluster.ValueString()
	servers, err := fetchAllServers(ctx, client, &cluster)
	if err != nil {
		return "", fmt.Errorf("listing servers after provision job: %w", err)
	}

	var match *cycle.Server
	for i := range servers {
		srv := &servers[i]
		if srv.LocationId != plan.LocationID.ValueString() || srv.ModelId != plan.ModelID.ValueString() {
			continue
		}
		if srv.State.Current == cycle.ServerStateCurrentDeleted || srv.State.Current == cycle.ServerStateCurrentDeleting {
			continue
		}
		if match == nil || srv.Events.Created.After(match.Events.Created) {
			match = srv
		}
	}
	if match == nil {
		return "", fmt.Errorf("provision job completed but no server matching cluster %q, location %q, model %q was found, and the job output did not include a server id",
			plan.Cluster.ValueString(), plan.LocationID.ValueString(), plan.ModelID.ValueString())
	}
	return match.Id, nil
}

func serverIDFromJob(job *cycle.Job) string {
	if job == nil {
		return ""
	}
	preferredKeys := []string{"server_id", "id", "server"}
	for _, task := range job.Tasks {
		if task.Output == nil {
			continue
		}
		for _, key := range preferredKeys {
			if value, ok := (*task.Output)[key]; ok && serverResourceIDPattern.MatchString(value) {
				return value
			}
		}
		for _, value := range *task.Output {
			if serverResourceIDPattern.MatchString(value) {
				return value
			}
		}
	}
	return ""
}

// serverDeleteJobID extracts a job id from DeleteServer. The generated client
// maps the documented 200 envelope onto JSON200, but the live API may return
// 202 with the same JobDescriptor body.
func serverDeleteJobID(resp *cycle.DeleteServerResponse) (string, bool) {
	if resp.JSON200 != nil {
		if resp.JSON200.Data.Job != nil {
			return resp.JSON200.Data.Job.Id, true
		}
		return "", true
	}
	switch resp.StatusCode() {
	case http.StatusOK, http.StatusAccepted:
		var envelope struct {
			Data cycle.JobDescriptor `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return "", false
		}
		if envelope.Data.Job != nil {
			return envelope.Data.Job.Id, true
		}
		return "", true
	default:
		return "", false
	}
}
