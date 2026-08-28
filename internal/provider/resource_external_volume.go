package provider

import (
	"context"
	"encoding/json"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewExternalVolumeResource)
}

var (
	_ resource.Resource                = (*externalVolumeResource)(nil)
	_ resource.ResourceWithConfigure   = (*externalVolumeResource)(nil)
	_ resource.ResourceWithImportState = (*externalVolumeResource)(nil)
)

// NewExternalVolumeResource returns the cycle_external_volume resource.
func NewExternalVolumeResource() resource.Resource {
	return &externalVolumeResource{}
}

type externalVolumeResource struct {
	client *CycleClient
}

type externalVolumeResourceModel struct {
	ID                 types.String                `tfsdk:"id"`
	Name               types.String                `tfsdk:"name"`
	Cluster            types.String                `tfsdk:"cluster"`
	LocationID         types.String                `tfsdk:"location_id"`
	ServerIDs          types.List                  `tfsdk:"server_ids"`
	Identifier         types.String                `tfsdk:"identifier"`
	Description        types.String                `tfsdk:"description"`
	Source             jsontypes.Normalized        `tfsdk:"source"`
	Attachment         jsontypes.Normalized        `tfsdk:"attachment"`
	Options            *externalVolumeOptionsModel `tfsdk:"options"`
	DeleteSourceDevice types.Bool                  `tfsdk:"delete_source_device"`
	Size               types.String                `tfsdk:"size"`
	State              types.String                `tfsdk:"state"`
	HubID              types.String                `tfsdk:"hub_id"`
}

type externalVolumeOptionsModel struct {
	Create *externalVolumeCreateOptionsModel `tfsdk:"create"`
}

type externalVolumeCreateOptionsModel struct {
	Size types.String `tfsdk:"size"`
}

func (r *externalVolumeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_volume"
}

func (r *externalVolumeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle external volume (`POST /v1/infrastructure/external-volumes`). " +
			"`source` and `attachment` are JSON objects matching the Cycle API unions " +
			"(`san-iscsi`, `ceph-rbd`, `aws-ebs` sources; `block` / `filesystem` attachments). " +
			"Those unions are too deep to flatten into native attributes, so they are stored as normalized JSON.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the external volume.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A custom name for the external volume.",
			},
			"cluster": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The identifier of the cluster this volume is associated with. Changing this forces a new volume to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The provider location ID where this volume exists. Changing this forces a new volume to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"server_ids": schema.ListAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Server IDs that this volume can be mounted on. Changing this forces a new volume to be created.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"identifier": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A human-readable slugged identifier for the volume. Generated from the name when omitted.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "A custom description for the external volume.",
			},
			"source": schema.StringAttribute{
				Required:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "JSON object describing the volume source. Discriminated by `type`: `san-iscsi` (`details.integration_ids`, `details.lun`), `ceph-rbd` (`details.integration_id`, `details.image`), or `aws-ebs` (`details.auth`, `details.volume`). Changing this forces a new volume to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attachment": schema.StringAttribute{
				Required:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "JSON object describing the attachment. Discriminated by `type`: `block` or `filesystem`, each with a `mode` (`single-node-writer`, `single-node-read-only`, `multi-node-writer`, …) and a `details` object. Changing this forces a new volume to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"options": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Configuration options controlling volume behavior.",
				Attributes: map[string]schema.Attribute{
					"create": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "When set, Cycle will attempt to create the backing volume on first attach if it does not already exist.",
						Attributes: map[string]schema.Attribute{
							"size": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Desired size of a newly created volume, as a Cycle `DataSize` string (e.g. `100GB`).",
							},
						},
					},
				},
			},
			"delete_source_device": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, also delete the underlying source device when this resource is destroyed. Used only on destroy. Defaults to `false`.",
			},
			"size": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current size of the volume as reported by the API. Null until the size has been determined.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current lifecycle state of the volume.",
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this volume belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *externalVolumeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *externalVolumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan externalVolumeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, ok := externalVolumeCreateBody(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}

	createResp, err := r.client.Client.CreateExternalVolumeWithResponse(ctx, &cycle.CreateExternalVolumeParams{}, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating external volume", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating external volume", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	externalVolumeResourceModelFromAPI(ctx, &plan, createResp.JSON201.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *externalVolumeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state externalVolumeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetExternalVolumeWithResponse(ctx, state.ID.ValueString(), &cycle.GetExternalVolumeParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading external volume", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading external volume", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	externalVolumeResourceModelFromAPI(ctx, &state, getResp.JSON200.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *externalVolumeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan externalVolumeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state externalVolumeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	description := plan.Description.ValueString()
	body := cycle.UpdateExternalVolumeJSONRequestBody{
		Name: &name,
		About: &struct {
			Description *string `json:"description,omitempty"`
		}{Description: &description},
		Options: externalVolumeOptionsToAPI(plan.Options),
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}

	updateResp, err := r.client.Client.UpdateExternalVolumeWithResponse(ctx, state.ID.ValueString(), &cycle.UpdateExternalVolumeParams{}, body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating external volume", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating external volume", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	externalVolumeResourceModelFromAPI(ctx, &plan, updateResp.JSON200.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *externalVolumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state externalVolumeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceDevice := state.DeleteSourceDevice.ValueBool()
	deleteResp, err := r.client.Client.DeleteExternalVolumeWithResponse(ctx, state.ID.ValueString(), cycle.DeleteExternalVolumeJSONRequestBody{
		Options: &struct {
			SourceDevice *bool `json:"source_device,omitempty"`
		}{SourceDevice: &sourceDevice},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting external volume", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting external volume", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}
	if job := deleteResp.JSON202.Data.Job; job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for external volume deletion", err.Error())
		}
	}
}

func (r *externalVolumeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func externalVolumeCreateBody(ctx context.Context, plan externalVolumeResourceModel, diags *diag.Diagnostics) (cycle.CreateExternalVolumeJSONRequestBody, bool) {
	var serverIDs []string
	diags.Append(plan.ServerIDs.ElementsAs(ctx, &serverIDs, false)...)
	if diags.HasError() {
		return cycle.CreateExternalVolumeJSONRequestBody{}, false
	}

	var source cycle.ExternalVolumeSource
	diags.Append(plan.Source.Unmarshal(&source)...)
	var attachment cycle.ExternalVolumeAttachment
	diags.Append(plan.Attachment.Unmarshal(&attachment)...)
	if diags.HasError() {
		return cycle.CreateExternalVolumeJSONRequestBody{}, false
	}

	body := cycle.CreateExternalVolumeJSONRequestBody{
		Name:       plan.Name.ValueString(),
		Cluster:    plan.Cluster.ValueString(),
		LocationId: plan.LocationID.ValueString(),
		ServerIds:  serverIDs,
		Source:     source,
		Attachment: attachment,
		About: &struct {
			Description string `json:"description"`
		}{Description: plan.Description.ValueString()},
	}
	if opts := externalVolumeOptionsToAPI(plan.Options); opts != nil {
		body.Options = *opts
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}
	return body, true
}

func externalVolumeOptionsToAPI(opts *externalVolumeOptionsModel) *cycle.ExternalVolumeOptions {
	if opts == nil {
		return &cycle.ExternalVolumeOptions{}
	}
	out := &cycle.ExternalVolumeOptions{}
	if opts.Create != nil && !opts.Create.Size.IsNull() && !opts.Create.Size.IsUnknown() {
		out.Create = &struct {
			Size cycle.DataSize `json:"size"`
		}{Size: opts.Create.Size.ValueString()}
	}
	return out
}

func externalVolumeResourceModelFromAPI(ctx context.Context, model *externalVolumeResourceModel, vol cycle.ExternalVolume, diags *diag.Diagnostics) {
	model.ID = types.StringValue(vol.Id)
	model.Name = types.StringValue(vol.Name)
	model.Cluster = types.StringValue(vol.Cluster)
	model.LocationID = types.StringValue(vol.LocationId)
	model.HubID = types.StringValue(vol.HubId)
	model.State = types.StringValue(string(vol.State.Current))
	model.Description = types.StringValue(vol.About.Description)
	if model.DeleteSourceDevice.IsNull() {
		model.DeleteSourceDevice = types.BoolValue(false)
	}

	if vol.Identifier != nil && *vol.Identifier != "" {
		model.Identifier = types.StringValue(*vol.Identifier)
	} else if model.Identifier.IsUnknown() {
		model.Identifier = types.StringNull()
	}

	ids, d := types.ListValueFrom(ctx, types.StringType, vol.ServerIds)
	diags.Append(d...)
	model.ServerIDs = ids

	if vol.Size != nil {
		model.Size = types.StringValue(*vol.Size)
	} else {
		model.Size = types.StringNull()
	}

	sourceJSON, err := json.Marshal(vol.Source)
	if err != nil {
		diags.AddError("Error encoding external volume source", err.Error())
	} else {
		model.Source = jsontypes.NewNormalizedValue(string(sourceJSON))
	}

	if vol.Attachment != nil {
		attachmentJSON, err := json.Marshal(vol.Attachment)
		if err != nil {
			diags.AddError("Error encoding external volume attachment", err.Error())
		} else {
			model.Attachment = jsontypes.NewNormalizedValue(string(attachmentJSON))
		}
	} else if model.Attachment.IsUnknown() {
		model.Attachment = jsontypes.NewNormalizedValue("{}")
	}

	model.Options = &externalVolumeOptionsModel{}
	if vol.Options.Create != nil {
		model.Options.Create = &externalVolumeCreateOptionsModel{
			Size: types.StringValue(vol.Options.Create.Size),
		}
	}
}
