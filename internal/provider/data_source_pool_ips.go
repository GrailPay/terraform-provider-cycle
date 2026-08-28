package provider

import (
	"context"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewPoolIPsDataSource)
}

var (
	_ datasource.DataSource              = (*poolIPsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*poolIPsDataSource)(nil)
)

// NewPoolIPsDataSource returns the cycle_pool_ips data source.
func NewPoolIPsDataSource() datasource.DataSource {
	return &poolIPsDataSource{}
}

type poolIPsDataSource struct {
	client *CycleClient
}

type poolIPsDataSourceModel struct {
	PoolID types.String      `tfsdk:"pool_id"`
	IPs    []poolIPDataModel `tfsdk:"ips"`
}

type poolIPDataModel struct {
	ID               types.String `tfsdk:"id"`
	Address          types.String `tfsdk:"address"`
	Kind             types.String `tfsdk:"kind"`
	CIDR             types.String `tfsdk:"cidr"`
	Gateway          types.String `tfsdk:"gateway"`
	State            types.String `tfsdk:"state"`
	ContainerID      types.String `tfsdk:"container_id"`
	EnvironmentID    types.String `tfsdk:"environment_id"`
	InstanceID       types.String `tfsdk:"instance_id"`
	VirtualMachineID types.String `tfsdk:"virtual_machine_id"`
}

func (d *poolIPsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool_ips"
}

func (d *poolIPsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the individual IP addresses in a Cycle IP pool (`GET /v1/infrastructure/ips/pools/{id}/ips`). " +
			"`cycle_ip_pools` only reports available/total counts; use this data source when you need the addresses themselves.",
		Attributes: map[string]schema.Attribute{
			"pool_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the IP pool whose addresses should be listed.",
			},
			"ips": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The addresses in the pool.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the IP.",
						},
						"address": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The IP address.",
						},
						"kind": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The IP protocol kind (`ipv4` or `ipv6`).",
						},
						"cidr": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The CIDR for this address.",
						},
						"gateway": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The gateway for this address.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the IP.",
						},
						"container_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The container this IP is assigned to, if any.",
						},
						"environment_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The environment this IP is assigned to, if any.",
						},
						"instance_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The instance this IP is assigned to, if any.",
						},
						"virtual_machine_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The virtual machine this IP is assigned to, if any.",
						},
					},
				},
			},
		},
	}
}

func (d *poolIPsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *poolIPsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config poolIPsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	poolID := config.PoolID.ValueString()
	apiResp, err := d.client.Client.GetPoolIPsWithResponse(ctx, poolID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing pool IPs", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"IP Pool Not Found",
			fmt.Sprintf("No IP pool with ID %q exists in this hub.", poolID),
		)
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "listing pool IPs", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	config.IPs = make([]poolIPDataModel, 0, len(apiResp.JSON200.Data))
	for _, ip := range apiResp.JSON200.Data {
		config.IPs = append(config.IPs, poolIPDataFromAPI(ip))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func poolIPDataFromAPI(ip cycle.Ip) poolIPDataModel {
	address := ip.Address
	if address == "" {
		address = ip.Ip
	}

	model := poolIPDataModel{
		ID:               types.StringValue(ip.Id),
		Address:          types.StringValue(address),
		Kind:             types.StringValue(string(ip.Kind)),
		CIDR:             types.StringValue(ip.Cidr),
		Gateway:          types.StringValue(ip.Gateway),
		State:            types.StringValue(string(ip.State.Current)),
		ContainerID:      types.StringNull(),
		EnvironmentID:    types.StringNull(),
		InstanceID:       types.StringNull(),
		VirtualMachineID: types.StringNull(),
	}
	if ip.Assignment != nil {
		model.ContainerID = types.StringValue(ip.Assignment.ContainerId)
		model.EnvironmentID = types.StringValue(ip.Assignment.EnvironmentId)
		model.InstanceID = types.StringValue(ip.Assignment.InstanceId)
		if ip.Assignment.VirtualMachine != nil {
			model.VirtualMachineID = types.StringValue(ip.Assignment.VirtualMachine.Id)
		}
	}
	return model
}
