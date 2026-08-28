package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewVPNDataSource)
}

var (
	_ datasource.DataSource              = (*vpnDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*vpnDataSource)(nil)
)

// NewVPNDataSource returns the cycle_vpn data source.
func NewVPNDataSource() datasource.DataSource {
	return &vpnDataSource{}
}

type vpnDataSource struct {
	client *CycleClient
}

type vpnDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	EnvironmentID    types.String `tfsdk:"environment_id"`
	Enable           types.Bool   `tfsdk:"enable"`
	AutoUpdate       types.Bool   `tfsdk:"auto_update"`
	AllowInternet    types.Bool   `tfsdk:"allow_internet"`
	CycleAccounts    types.Bool   `tfsdk:"cycle_accounts"`
	VPNAccounts      types.Bool   `tfsdk:"vpn_accounts"`
	Webhook          types.String `tfsdk:"webhook"`
	CustomDirectives types.String `tfsdk:"custom_directives"`
	URL              types.String `tfsdk:"url"`
}

func (d *vpnDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpn"
}

func (d *vpnDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the current VPN service configuration for a Cycle environment. The VPN is an environment singleton. " +
			"`high_availability` is not returned by the API and is omitted here.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The environment ID this VPN belongs to.",
			},
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose VPN should be read.",
			},
			"enable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the VPN service is currently enabled.",
			},
			"auto_update": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the VPN service container is set to auto-update.",
			},
			"allow_internet": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether VPN clients can reach the public internet through the service.",
			},
			"cycle_accounts": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether hub member accounts can authenticate to the VPN.",
			},
			"vpn_accounts": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether custom VPN user accounts can authenticate to the VPN.",
			},
			"webhook": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The webhook URL used for VPN authentication, if configured.",
			},
			"custom_directives": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Custom OpenVPN directives configured on the service, if any.",
			},
			"url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The connection URL for the VPN service.",
			},
		},
	}
}

func (d *vpnDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *vpnDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		return
	}

	var config vpnDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	environmentID := config.EnvironmentID.ValueString()
	apiResp, err := d.client.Client.GetVPNServiceWithResponse(ctx, environmentID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading VPN", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"VPN Not Found",
			fmt.Sprintf("No VPN service was found for environment %q.", environmentID),
		)
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading VPN", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	resourceModel := vpnResourceModel{
		EnvironmentID: config.EnvironmentID,
	}
	applyVPNInfo(&resourceModel, apiResp.JSON200.Data)

	config.ID = resourceModel.ID
	config.Enable = resourceModel.Enable
	config.AutoUpdate = resourceModel.AutoUpdate
	config.AllowInternet = resourceModel.AllowInternet
	config.CycleAccounts = resourceModel.CycleAccounts
	config.VPNAccounts = resourceModel.VPNAccounts
	config.Webhook = resourceModel.Webhook
	config.CustomDirectives = resourceModel.CustomDirectives
	config.URL = resourceModel.URL

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
