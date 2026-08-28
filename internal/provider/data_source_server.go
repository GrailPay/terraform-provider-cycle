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
	RegisterDataSource(NewServerDataSource)
}

var (
	_ datasource.DataSource              = (*serverDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serverDataSource)(nil)
)

// NewServerDataSource returns the cycle_server data source.
func NewServerDataSource() datasource.DataSource {
	return &serverDataSource{}
}

type serverDataSource struct {
	client *CycleClient
}

type serverDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Hostname      types.String `tfsdk:"hostname"`
	Cluster       types.String `tfsdk:"cluster"`
	State         types.String `tfsdk:"state"`
	LocationID    types.String `tfsdk:"location_id"`
	ModelID       types.String `tfsdk:"model_id"`
	IntegrationID types.String `tfsdk:"integration_id"`
	HubID         types.String `tfsdk:"hub_id"`
	Nickname      types.String `tfsdk:"nickname"`
	IPs           types.List   `tfsdk:"ips"`
}

func (d *serverDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (d *serverDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle server by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique ID of the server.",
			},
			"hostname": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The hostname assigned to the server.",
			},
			"cluster": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The identifier of the cluster the server is deployed into.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the server.",
			},
			"location_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The provider location ID of the server.",
			},
			"model_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The provider server model ID.",
			},
			"integration_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The provider integration ID used to provision the server.",
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this server belongs to.",
			},
			"nickname": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "A custom display name for the server, if one has been set.",
			},
			"ips": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IP addresses assigned during provisioning (`provider.init_ips`).",
			},
		},
	}
}

func (d *serverDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *serverDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config serverDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Client.GetServerWithResponse(ctx, config.ID.ValueString(), &cycle.GetServerParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading server", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Server Not Found",
			fmt.Sprintf("No server with ID %q exists in this hub.", config.ID.ValueString()),
		)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading server", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	srv := getResp.JSON200.Data
	config.ID = types.StringValue(srv.Id)
	config.Hostname = types.StringValue(srv.Hostname)
	config.Cluster = types.StringValue(srv.Cluster)
	config.State = types.StringValue(string(srv.State.Current))
	config.LocationID = types.StringValue(srv.LocationId)
	config.ModelID = types.StringValue(srv.ModelId)
	config.IntegrationID = types.StringValue(srv.Provider.IntegrationId)
	config.HubID = types.StringValue(srv.HubId)
	if srv.Nickname != nil {
		config.Nickname = types.StringValue(*srv.Nickname)
	} else {
		config.Nickname = types.StringNull()
	}

	ips := []string{}
	if srv.Provider.InitIps != nil {
		ips = *srv.Provider.InitIps
	}
	ipList, diags := types.ListValueFrom(ctx, types.StringType, ips)
	resp.Diagnostics.Append(diags...)
	config.IPs = ipList

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
