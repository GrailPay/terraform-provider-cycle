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
	RegisterDataSource(NewVPNUsersDataSource)
}

var (
	_ datasource.DataSource              = (*vpnUsersDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*vpnUsersDataSource)(nil)
)

// NewVPNUsersDataSource returns the cycle_vpn_users data source.
func NewVPNUsersDataSource() datasource.DataSource {
	return &vpnUsersDataSource{}
}

type vpnUsersDataSource struct {
	client *CycleClient
}

type vpnUsersDataSourceModel struct {
	EnvironmentID types.String           `tfsdk:"environment_id"`
	Users         []vpnUserDataItemModel `tfsdk:"users"`
}

type vpnUserDataItemModel struct {
	ID        types.String `tfsdk:"id"`
	Username  types.String `tfsdk:"username"`
	LastLogin types.String `tfsdk:"last_login"`
}

func (d *vpnUsersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpn_users"
}

func (d *vpnUsersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists custom VPN users on an environment's VPN service. Passwords are write-only on create and are never returned.",
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose VPN users should be listed.",
			},
			"users": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The VPN users configured on the environment.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the VPN user.",
						},
						"username": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The VPN login username.",
						},
						"last_login": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "RFC3339 timestamp of the last time this user logged into the VPN, if any.",
						},
					},
				},
			},
		},
	}
}

func (d *vpnUsersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *vpnUsersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		return
	}

	var config vpnUsersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	environmentID := config.EnvironmentID.ValueString()
	apiResp, err := d.client.Client.GetVPNUsersWithResponse(ctx, environmentID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing VPN users", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"VPN Users Not Found",
			fmt.Sprintf("No VPN service was found for environment %q.", environmentID),
		)
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "listing VPN users", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	config.Users = make([]vpnUserDataItemModel, 0, len(apiResp.JSON200.Data))
	for i := range apiResp.JSON200.Data {
		var item vpnUserResourceModel
		item.EnvironmentID = config.EnvironmentID
		vpnUserModelFromAPI(&item, apiResp.JSON200.Data[i])
		config.Users = append(config.Users, vpnUserDataItemModel{
			ID:        item.ID,
			Username:  item.Username,
			LastLogin: item.LastLogin,
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
