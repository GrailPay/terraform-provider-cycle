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
	RegisterDataSource(NewIPPoolDataSource)
}

var (
	_ datasource.DataSource              = (*ipPoolDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*ipPoolDataSource)(nil)
)

// NewIPPoolDataSource returns the cycle_ip_pool data source.
func NewIPPoolDataSource() datasource.DataSource {
	return &ipPoolDataSource{}
}

type ipPoolDataSource struct {
	client *CycleClient
}

type ipPoolDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	CIDR         types.String `tfsdk:"cidr"`
	Gateway      types.String `tfsdk:"gateway"`
	Kind         types.String `tfsdk:"kind"`
	Floating     types.Bool   `tfsdk:"floating"`
	LocationID   types.String `tfsdk:"location_id"`
	ServerID     types.String `tfsdk:"server_id"`
	IPsAvailable types.Int64  `tfsdk:"ips_available"`
	IPsTotal     types.Int64  `tfsdk:"ips_total"`
	State        types.String `tfsdk:"state"`
}

func (d *ipPoolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_pool"
}

func (d *ipPoolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := ipPoolDataAttributes()
	attrs["id"] = schema.StringAttribute{
		Required:            true,
		MarkdownDescription: "The unique ID of the IP pool.",
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle IP pool by ID. This data source reports available/total IP counts; " +
			"use `cycle_pool_ips` to enumerate individual addresses.",
		Attributes: attrs,
	}
}

func (d *ipPoolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *ipPoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ipPoolDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Client.GetIPPoolWithResponse(ctx, config.ID.ValueString(), &cycle.GetIPPoolParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading IP pool", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"IP Pool Not Found",
			fmt.Sprintf("No IP pool with ID %q exists in this hub.", config.ID.ValueString()),
		)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading IP pool", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	model := ipPoolDataModelFromAPI(getResp.JSON200.Data)
	config.ID = model.ID
	config.CIDR = model.CIDR
	config.Gateway = model.Gateway
	config.Kind = model.Kind
	config.Floating = model.Floating
	config.LocationID = model.LocationID
	config.ServerID = model.ServerID
	config.IPsAvailable = model.IPsAvailable
	config.IPsTotal = model.IPsTotal
	config.State = model.State

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
