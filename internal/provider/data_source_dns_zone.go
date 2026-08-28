package provider

import (
	"context"
	"fmt"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewDnsZoneDataSource)
}

var (
	_ datasource.DataSource                     = (*dnsZoneDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*dnsZoneDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*dnsZoneDataSource)(nil)
)

// NewDnsZoneDataSource returns the cycle_dns_zone data source.
func NewDnsZoneDataSource() datasource.DataSource {
	return &dnsZoneDataSource{}
}

type dnsZoneDataSource struct {
	client *CycleClient
}

type dnsZoneDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	Origin types.String `tfsdk:"origin"`
	Hosted types.Bool   `tfsdk:"hosted"`
	State  types.String `tfsdk:"state"`
}

func (d *dnsZoneDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone"
}

func (d *dnsZoneDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle DNS zone by ID or by origin (domain name). Exactly one of `id` or " +
			"`origin` must be set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique ID of the DNS zone. Conflicts with `origin`.",
			},
			"origin": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The origin (domain name) of the DNS zone, e.g. `example.com`. Conflicts with `id`.",
			},
			"hosted": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this zone is hosted by Cycle (`true`) or linked (`false`).",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the DNS zone.",
			},
		},
	}
}

func (d *dnsZoneDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("origin"),
		),
	}
}

func (d *dnsZoneDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *dnsZoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config dnsZoneDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var zone *cycle.DnsZone
	if !config.ID.IsNull() {
		apiResp, err := d.client.Client.GetDNSZoneWithResponse(ctx, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading DNS zone", err.Error())
			return
		}
		if apiResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "reading DNS zone", apiResp.StatusCode(), apiResp.JSONDefault)
			return
		}
		zone = &apiResp.JSON200.Data
	} else {
		found, diags := d.findZoneByOrigin(ctx, config.Origin.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		zone = found
	}

	config.ID = types.StringValue(zone.Id)
	config.Origin = types.StringValue(zone.Origin)
	config.Hosted = types.BoolValue(zone.Hosted)
	config.State = types.StringValue(string(zone.State.Current))
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// findZoneByOrigin pages through all DNS zones looking for an exact origin match.
func (d *dnsZoneDataSource) findZoneByOrigin(ctx context.Context, origin string) (*cycle.DnsZone, diag.Diagnostics) {
	var diags diag.Diagnostics

	pageSize := float32(100)
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := pageSize
		apiResp, err := d.client.Client.GetDNSZonesWithResponse(ctx, &cycle.GetDNSZonesParams{
			Page: &cycle.PageParam{Number: &number, Size: &size},
		})
		if err != nil {
			diags.AddError("Error listing DNS zones", err.Error())
			return nil, diags
		}
		if apiResp.JSON200 == nil {
			addAPIError(&diags, "listing DNS zones", apiResp.StatusCode(), apiResp.JSONDefault)
			return nil, diags
		}

		for i := range apiResp.JSON200.Data {
			if apiResp.JSON200.Data[i].Origin == origin {
				return &apiResp.JSON200.Data[i], diags
			}
		}

		if len(apiResp.JSON200.Data) < int(pageSize) {
			break
		}
	}

	diags.AddError(
		"DNS Zone Not Found",
		fmt.Sprintf("No DNS zone with origin %q exists in this hub.", origin),
	)
	return nil, diags
}
