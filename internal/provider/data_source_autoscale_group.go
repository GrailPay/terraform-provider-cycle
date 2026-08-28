package provider

import (
	"context"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewAutoscaleGroupDataSource)
}

var (
	_ datasource.DataSource                     = (*autoscaleGroupDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*autoscaleGroupDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*autoscaleGroupDataSource)(nil)
)

// NewAutoscaleGroupDataSource returns the cycle_autoscale_group data source.
func NewAutoscaleGroupDataSource() datasource.DataSource {
	return &autoscaleGroupDataSource{}
}

type autoscaleGroupDataSource struct {
	client *CycleClient
}

type autoscaleGroupDataSourceModel struct {
	ID             types.String                        `tfsdk:"id"`
	Name           types.String                        `tfsdk:"name"`
	Identifier     types.String                        `tfsdk:"identifier"`
	Cluster        types.String                        `tfsdk:"cluster"`
	Infrastructure []autoscaleGroupInfrastructureModel `tfsdk:"infrastructure"`
	Scale          *autoscaleGroupScaleModel           `tfsdk:"scale"`
	HubID          types.String                        `tfsdk:"hub_id"`
	State          types.String                        `tfsdk:"state"`
}

func (d *autoscaleGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_autoscale_group"
}

func (d *autoscaleGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle auto-scale group by ID or identifier.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique ID of the auto-scale group. Exactly one of `id` or `identifier` must be set.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The user-defined name of the auto-scale group.",
			},
			"identifier": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The human-readable slugged identifier of the auto-scale group. Exactly one of `id` or `identifier` must be set.",
			},
			"cluster": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The identifier of the cluster this auto-scale group belongs to.",
			},
			"infrastructure": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Server models Cycle may provision from.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"provider": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The infrastructure provider identifier.",
						},
						"model_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The provider server model ID.",
						},
						"integration_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The provider integration ID associated with this model.",
						},
						"priority": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Relative priority of this model.",
						},
						"locations": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Locations this model may be provisioned in.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The provider location ID.",
									},
									"availability_zones": schema.ListAttribute{
										Computed:            true,
										ElementType:         types.StringType,
										MarkdownDescription: "Availability zones within the location.",
									},
								},
							},
						},
					},
				},
			},
			"scale": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Scale-up and scale-down bounds for the group.",
				Attributes: map[string]schema.Attribute{
					"up": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Scale-up settings.",
						Attributes: map[string]schema.Attribute{
							"maximum": schema.Int64Attribute{
								Computed:            true,
								MarkdownDescription: "Maximum number of servers this group may provision.",
							},
						},
					},
					"down": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Scale-down settings.",
						Attributes: map[string]schema.Attribute{
							"inactivity_period": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "How long after the last instance is deployed before a server is eligible for deletion.",
							},
							"method": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Which server to remove first when scaling down (`fifo` or `lifo`).",
							},
							"min_ttl": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Minimum time-to-live for a server provisioned by an autoscale event.",
							},
						},
					},
				},
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this auto-scale group belongs to.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the auto-scale group.",
			},
		},
	}
}

func (d *autoscaleGroupDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("identifier"),
		),
	}
}

func (d *autoscaleGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *autoscaleGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config autoscaleGroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var group cycle.AutoScaleGroup
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		getResp, err := d.client.Client.GetAutoScaleGroupWithResponse(ctx, config.ID.ValueString(), &cycle.GetAutoScaleGroupParams{})
		if err != nil {
			resp.Diagnostics.AddError("Error reading auto-scale group", err.Error())
			return
		}
		if getResp.StatusCode() == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Auto-Scale Group Not Found",
				fmt.Sprintf("No auto-scale group with ID %q exists in this hub.", config.ID.ValueString()),
			)
			return
		}
		if getResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "reading auto-scale group", getResp.StatusCode(), getResp.JSONDefault)
			return
		}
		group = getResp.JSON200.Data
	} else {
		identifier := config.Identifier.ValueString()
		groups, err := fetchAutoScaleGroupsByIdentifier(ctx, d.client, identifier)
		if err != nil {
			resp.Diagnostics.AddError("Error listing auto-scale groups", err.Error())
			return
		}
		switch len(groups) {
		case 0:
			resp.Diagnostics.AddError(
				"Auto-Scale Group Not Found",
				fmt.Sprintf("No auto-scale group with identifier %q exists in this hub.", identifier),
			)
			return
		case 1:
			group = groups[0]
		default:
			resp.Diagnostics.AddError(
				"Multiple Auto-Scale Groups Found",
				fmt.Sprintf("Found %d auto-scale groups with identifier %q. Use the id attribute to select one unambiguously.", len(groups), identifier),
			)
			return
		}
	}

	config.ID = types.StringValue(group.Id)
	config.Name = types.StringValue(group.Name)
	config.Identifier = types.StringValue(group.Identifier)
	config.Cluster = types.StringValue(group.Cluster)
	config.HubID = types.StringValue(group.HubId)
	config.State = types.StringValue(string(group.State.Current))
	config.Infrastructure = autoscaleGroupInfrastructureFromAPI(ctx, group.Infrastructure, &resp.Diagnostics)
	config.Scale = autoscaleGroupScaleFromAPI(group.Scale)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func fetchAutoScaleGroupsByIdentifier(ctx context.Context, client *CycleClient, identifier string) ([]cycle.AutoScaleGroup, error) {
	const pageSize = 100

	var all []cycle.AutoScaleGroup
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetAutoScaleGroupsWithResponse(ctx, &cycle.GetAutoScaleGroupsParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
			Filter: &struct {
				Cluster    *string `json:"cluster,omitempty"`
				Identifier *string `json:"identifier,omitempty"`
				Search     *string `json:"search,omitempty"`
				State      *string `json:"state,omitempty"`
			}{Identifier: &identifier},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing auto-scale groups", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
