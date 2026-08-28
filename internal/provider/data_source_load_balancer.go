package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewLoadBalancerDataSource)
}

var (
	_ datasource.DataSource              = (*loadBalancerDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*loadBalancerDataSource)(nil)
)

// NewLoadBalancerDataSource returns the cycle_load_balancer data source.
func NewLoadBalancerDataSource() datasource.DataSource {
	return &loadBalancerDataSource{}
}

type loadBalancerDataSource struct {
	client *CycleClient
}

type loadBalancerDataSourceModel struct {
	ID               types.String         `tfsdk:"id"`
	EnvironmentID    types.String         `tfsdk:"environment_id"`
	HighAvailability types.Bool           `tfsdk:"high_availability"`
	AutoUpdate       types.Bool           `tfsdk:"auto_update"`
	Config           jsontypes.Normalized `tfsdk:"config"`
	Enable           types.Bool           `tfsdk:"enable"`
	ContainerID      types.String         `tfsdk:"container_id"`
	CurrentType      types.String         `tfsdk:"current_type"`
}

func (d *loadBalancerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_load_balancer"
}

func (d *loadBalancerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the current load balancer service configuration for a Cycle environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The environment ID this load balancer belongs to.",
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose load balancer should be read.",
			},
			"high_availability": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the load balancer service runs in high availability mode.",
			},
			"auto_update": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the load balancer service container is set to auto-update.",
			},
			"config": schema.StringAttribute{
				Computed:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "The current load balancer configuration as a normalized JSON object.",
			},
			"enable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the load balancer service is currently enabled.",
			},
			"container_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the load balancer service container, if one has been provisioned.",
			},
			"current_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The load balancer implementation currently in use (`haproxy` or `v1`).",
			},
		},
	}
}

func (d *loadBalancerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *loadBalancerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		return
	}

	var config loadBalancerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.Client.GetLoadBalancerServiceWithResponse(ctx, config.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading load balancer", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Load Balancer Not Found",
			fmt.Sprintf("No load balancer service was found for environment %q.", config.EnvironmentID.ValueString()),
		)
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading load balancer", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	resourceModel := loadBalancerResourceModel{
		EnvironmentID: config.EnvironmentID,
	}
	resp.Diagnostics.Append(applyLoadBalancerInfo(&resourceModel, apiResp.JSON200.Data, true)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.ID = resourceModel.ID
	config.HighAvailability = resourceModel.HighAvailability
	config.AutoUpdate = resourceModel.AutoUpdate
	config.Config = resourceModel.Config
	config.Enable = resourceModel.Enable
	config.ContainerID = resourceModel.ContainerID
	config.CurrentType = resourceModel.CurrentType

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
