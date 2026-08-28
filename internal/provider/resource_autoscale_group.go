package provider

import (
	"context"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewAutoscaleGroupResource)
}

var (
	_ resource.Resource                = (*autoscaleGroupResource)(nil)
	_ resource.ResourceWithConfigure   = (*autoscaleGroupResource)(nil)
	_ resource.ResourceWithImportState = (*autoscaleGroupResource)(nil)
)

// NewAutoscaleGroupResource returns the cycle_autoscale_group resource.
func NewAutoscaleGroupResource() resource.Resource {
	return &autoscaleGroupResource{}
}

type autoscaleGroupResource struct {
	client *CycleClient
}

type autoscaleGroupResourceModel struct {
	ID             types.String                        `tfsdk:"id"`
	Name           types.String                        `tfsdk:"name"`
	Identifier     types.String                        `tfsdk:"identifier"`
	Cluster        types.String                        `tfsdk:"cluster"`
	Infrastructure []autoscaleGroupInfrastructureModel `tfsdk:"infrastructure"`
	Scale          *autoscaleGroupScaleModel           `tfsdk:"scale"`
	HubID          types.String                        `tfsdk:"hub_id"`
	State          types.String                        `tfsdk:"state"`
}

type autoscaleGroupInfrastructureModel struct {
	Provider      types.String                  `tfsdk:"provider"`
	ModelID       types.String                  `tfsdk:"model_id"`
	IntegrationID types.String                  `tfsdk:"integration_id"`
	Priority      types.Int64                   `tfsdk:"priority"`
	Locations     []autoscaleGroupLocationModel `tfsdk:"locations"`
}

type autoscaleGroupLocationModel struct {
	ID                types.String `tfsdk:"id"`
	AvailabilityZones types.List   `tfsdk:"availability_zones"`
}

type autoscaleGroupScaleModel struct {
	Up   *autoscaleGroupScaleUpModel   `tfsdk:"up"`
	Down *autoscaleGroupScaleDownModel `tfsdk:"down"`
}

type autoscaleGroupScaleUpModel struct {
	Maximum types.Int64 `tfsdk:"maximum"`
}

type autoscaleGroupScaleDownModel struct {
	InactivityPeriod types.String `tfsdk:"inactivity_period"`
	Method           types.String `tfsdk:"method"`
	MinTTL           types.String `tfsdk:"min_ttl"`
}

func (r *autoscaleGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_autoscale_group"
}

func (r *autoscaleGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyStringList := types.ListValueMust(types.StringType, []attr.Value{})

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle auto-scale group (`POST /v1/infrastructure/auto-scale/groups`). " +
			"The group describes which server models and locations Cycle may provision from, and the scale-up / scale-down bounds.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the auto-scale group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A user-defined name for the auto-scale group.",
			},
			"identifier": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A human-readable slugged identifier for the auto-scale group.",
			},
			"cluster": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The identifier of the cluster this auto-scale group belongs to. Changing this forces a new group to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"infrastructure": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Ordered list of server models Cycle may provision from. Lower `priority` values are preferred.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"provider": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The infrastructure provider identifier (e.g. the vendor slug used by the integration).",
						},
						"model_id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The provider server model ID to provision.",
						},
						"integration_id": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "The provider integration ID associated with this model.",
						},
						"priority": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "Relative priority of this model. Lower numbers are chosen first.",
						},
						"locations": schema.ListNestedAttribute{
							Required:            true,
							MarkdownDescription: "Locations (and optional availability zones) this model may be provisioned in.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The provider location ID.",
									},
									"availability_zones": schema.ListAttribute{
										Optional:            true,
										Computed:            true,
										ElementType:         types.StringType,
										Default:             listdefault.StaticValue(emptyStringList),
										MarkdownDescription: "Availability zones within the location. An empty list means any zone.",
									},
								},
							},
						},
					},
				},
			},
			"scale": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Scale-up and scale-down bounds for the group.",
				Attributes: map[string]schema.Attribute{
					"up": schema.SingleNestedAttribute{
						Required:            true,
						MarkdownDescription: "Scale-up settings.",
						Attributes: map[string]schema.Attribute{
							"maximum": schema.Int64Attribute{
								Required:            true,
								MarkdownDescription: "Maximum number of servers this group may provision.",
							},
						},
					},
					"down": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Scale-down settings. Duration fields use Cycle duration strings (e.g. `1h`, `24h`).",
						Attributes: map[string]schema.Attribute{
							"inactivity_period": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "How long after the last instance is deployed before a server is eligible for deletion.",
							},
							"method": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Which server to remove first when scaling down. One of `fifo` or `lifo`.",
								Validators: []validator.String{
									stringvalidator.OneOf("fifo", "lifo"),
								},
							},
							"min_ttl": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Minimum time-to-live for a server provisioned by an autoscale event.",
							},
						},
					},
				},
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this auto-scale group belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the auto-scale group.",
			},
		},
	}
}

func (r *autoscaleGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *autoscaleGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan autoscaleGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	infra, scale, ok := autoscaleGroupPlanToAPI(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}

	createResp, err := r.client.Client.CreateAutoScaleGroupWithResponse(ctx, cycle.CreateAutoScaleGroupJSONRequestBody{
		Name:           plan.Name.ValueString(),
		Identifier:     plan.Identifier.ValueString(),
		Cluster:        plan.Cluster.ValueString(),
		Infrastructure: infra,
		Scale:          scale,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating auto-scale group", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating auto-scale group", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	autoscaleGroupResourceModelFromAPI(ctx, &plan, createResp.JSON201.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *autoscaleGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state autoscaleGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetAutoScaleGroupWithResponse(ctx, state.ID.ValueString(), &cycle.GetAutoScaleGroupParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading auto-scale group", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading auto-scale group", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	autoscaleGroupResourceModelFromAPI(ctx, &state, getResp.JSON200.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *autoscaleGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan autoscaleGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state autoscaleGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	infra, scale, ok := autoscaleGroupPlanToAPI(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}

	name := plan.Name.ValueString()
	identifier := plan.Identifier.ValueString()
	updateResp, err := r.client.Client.UpdateAutoScaleGroupWithResponse(ctx, state.ID.ValueString(), cycle.UpdateAutoScaleGroupJSONRequestBody{
		Name:           &name,
		Identifier:     &identifier,
		Infrastructure: &infra,
		Scale:          &scale,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating auto-scale group", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating auto-scale group", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	autoscaleGroupResourceModelFromAPI(ctx, &plan, updateResp.JSON200.Data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *autoscaleGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state autoscaleGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteAutoScaleGroupWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting auto-scale group", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting auto-scale group", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}
	if job := deleteResp.JSON202.Data.Job; job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for auto-scale group deletion", err.Error())
		}
	}
}

func (r *autoscaleGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func autoscaleGroupPlanToAPI(ctx context.Context, plan autoscaleGroupResourceModel, diags *diag.Diagnostics) (cycle.AutoScaleGroupInfrastructure, cycle.AutoScaleGroupScale, bool) {
	infra := cycle.AutoScaleGroupInfrastructure{
		Models: make([]struct {
			IntegrationId *cycle.ID `json:"integration_id,omitempty"`
			Locations     []struct {
				AvailabilityZones []string `json:"availability_zones"`
				Id                string   `json:"id"`
			} `json:"locations"`
			ModelId  string `json:"model_id"`
			Priority int    `json:"priority"`
			Provider string `json:"provider"`
		}, 0, len(plan.Infrastructure)),
	}

	for _, model := range plan.Infrastructure {
		item := struct {
			IntegrationId *cycle.ID `json:"integration_id,omitempty"`
			Locations     []struct {
				AvailabilityZones []string `json:"availability_zones"`
				Id                string   `json:"id"`
			} `json:"locations"`
			ModelId  string `json:"model_id"`
			Priority int    `json:"priority"`
			Provider string `json:"provider"`
		}{
			ModelId:  model.ModelID.ValueString(),
			Priority: int(model.Priority.ValueInt64()),
			Provider: model.Provider.ValueString(),
		}
		if !model.IntegrationID.IsNull() && !model.IntegrationID.IsUnknown() {
			id := cycle.ID(model.IntegrationID.ValueString())
			item.IntegrationId = &id
		}
		for _, loc := range model.Locations {
			zones := []string{}
			if !loc.AvailabilityZones.IsNull() && !loc.AvailabilityZones.IsUnknown() {
				diags.Append(loc.AvailabilityZones.ElementsAs(ctx, &zones, false)...)
				if zones == nil {
					zones = []string{}
				}
			}
			item.Locations = append(item.Locations, struct {
				AvailabilityZones []string `json:"availability_zones"`
				Id                string   `json:"id"`
			}{
				AvailabilityZones: zones,
				Id:                loc.ID.ValueString(),
			})
		}
		infra.Models = append(infra.Models, item)
	}

	scale := cycle.AutoScaleGroupScale{}
	if plan.Scale != nil && plan.Scale.Up != nil && !plan.Scale.Up.Maximum.IsNull() && !plan.Scale.Up.Maximum.IsUnknown() {
		maximum := int(plan.Scale.Up.Maximum.ValueInt64())
		scale.Up = &struct {
			Maximum *int `json:"maximum,omitempty"`
		}{Maximum: &maximum}
	}
	if plan.Scale != nil && plan.Scale.Down != nil {
		down := &struct {
			InactivityPeriod *cycle.Duration                      `json:"inactivity_period,omitempty"`
			Method           *cycle.AutoScaleGroupScaleDownMethod `json:"method,omitempty"`
			MinTtl           *cycle.Duration                      `json:"min_ttl,omitempty"`
		}{}
		if !plan.Scale.Down.InactivityPeriod.IsNull() && !plan.Scale.Down.InactivityPeriod.IsUnknown() {
			period := plan.Scale.Down.InactivityPeriod.ValueString()
			down.InactivityPeriod = &period
		}
		if !plan.Scale.Down.Method.IsNull() && !plan.Scale.Down.Method.IsUnknown() {
			method := cycle.AutoScaleGroupScaleDownMethod(plan.Scale.Down.Method.ValueString())
			down.Method = &method
		}
		if !plan.Scale.Down.MinTTL.IsNull() && !plan.Scale.Down.MinTTL.IsUnknown() {
			ttl := plan.Scale.Down.MinTTL.ValueString()
			down.MinTtl = &ttl
		}
		scale.Down = down
	}

	return infra, scale, !diags.HasError()
}

func autoscaleGroupResourceModelFromAPI(ctx context.Context, model *autoscaleGroupResourceModel, group cycle.AutoScaleGroup, diags *diag.Diagnostics) {
	model.ID = types.StringValue(group.Id)
	model.Name = types.StringValue(group.Name)
	model.Identifier = types.StringValue(group.Identifier)
	model.Cluster = types.StringValue(group.Cluster)
	model.HubID = types.StringValue(group.HubId)
	model.State = types.StringValue(string(group.State.Current))
	model.Infrastructure = autoscaleGroupInfrastructureFromAPI(ctx, group.Infrastructure, diags)
	model.Scale = autoscaleGroupScaleFromAPI(group.Scale)
}

func autoscaleGroupInfrastructureFromAPI(ctx context.Context, infra cycle.AutoScaleGroupInfrastructure, diags *diag.Diagnostics) []autoscaleGroupInfrastructureModel {
	out := make([]autoscaleGroupInfrastructureModel, 0, len(infra.Models))
	for _, model := range infra.Models {
		item := autoscaleGroupInfrastructureModel{
			Provider: types.StringValue(model.Provider),
			ModelID:  types.StringValue(model.ModelId),
			Priority: types.Int64Value(int64(model.Priority)),
		}
		if model.IntegrationId != nil {
			item.IntegrationID = types.StringValue(*model.IntegrationId)
		} else {
			item.IntegrationID = types.StringNull()
		}
		item.Locations = make([]autoscaleGroupLocationModel, 0, len(model.Locations))
		for _, loc := range model.Locations {
			zones := loc.AvailabilityZones
			if zones == nil {
				zones = []string{}
			}
			zoneList, d := types.ListValueFrom(ctx, types.StringType, zones)
			diags.Append(d...)
			item.Locations = append(item.Locations, autoscaleGroupLocationModel{
				ID:                types.StringValue(loc.Id),
				AvailabilityZones: zoneList,
			})
		}
		out = append(out, item)
	}
	return out
}

func autoscaleGroupScaleFromAPI(scale *cycle.AutoScaleGroupScale) *autoscaleGroupScaleModel {
	out := &autoscaleGroupScaleModel{
		Up: &autoscaleGroupScaleUpModel{
			Maximum: types.Int64Null(),
		},
	}
	if scale == nil {
		return out
	}
	if scale.Up != nil && scale.Up.Maximum != nil {
		out.Up.Maximum = types.Int64Value(int64(*scale.Up.Maximum))
	}
	if scale.Down != nil {
		down := &autoscaleGroupScaleDownModel{
			InactivityPeriod: types.StringNull(),
			Method:           types.StringNull(),
			MinTTL:           types.StringNull(),
		}
		if scale.Down.InactivityPeriod != nil {
			down.InactivityPeriod = types.StringValue(*scale.Down.InactivityPeriod)
		}
		if scale.Down.Method != nil {
			down.Method = types.StringValue(string(*scale.Down.Method))
		}
		if scale.Down.MinTtl != nil {
			down.MinTTL = types.StringValue(*scale.Down.MinTtl)
		}
		out.Down = down
	}
	return out
}
