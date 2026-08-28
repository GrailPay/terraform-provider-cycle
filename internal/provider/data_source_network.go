package provider

import (
	"context"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewNetworkDataSource)
}

var (
	_ datasource.DataSource              = (*networkDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*networkDataSource)(nil)
)

// NewNetworkDataSource returns the cycle_network data source.
func NewNetworkDataSource() datasource.DataSource {
	return &networkDataSource{}
}

type networkDataSource struct {
	client *CycleClient
}

func (d *networkDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (d *networkDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle SDN network by ID.",
		Attributes:          networkDataSourceAttributes(true),
	}
}

func (d *networkDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *networkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config networkModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Client.GetNetworkWithResponse(ctx, config.ID.ValueString(), &cycle.GetNetworkParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading network", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Network Not Found",
			fmt.Sprintf("No SDN network with ID %q exists in this hub.", config.ID.ValueString()),
		)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading network", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(networkModelFromAPI(ctx, &config, getResp.JSON200.Data, true)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func networkDataSourceAttributes(idRequired bool) map[string]schema.Attribute {
	id := schema.StringAttribute{
		MarkdownDescription: "The unique ID of the SDN network.",
	}
	if idRequired {
		id.Required = true
	} else {
		id.Computed = true
	}

	return map[string]schema.Attribute{
		"id": id,
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The user-defined name of the network.",
		},
		"identifier": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The network identifier used to construct HTTP calls that specifically use this network.",
		},
		"cluster": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The identifier of the cluster whose environments may join this network.",
		},
		"environment_ids": schema.SetAttribute{
			Computed:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "Environment IDs currently attached to this network.",
		},
		"acl": schema.StringAttribute{
			Computed:            true,
			CustomType:          jsontypes.NormalizedType{},
			MarkdownDescription: "The network ACL as a normalized JSON object, or null when unset.",
		},
		"hub_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the hub this network belongs to.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The current state of the network.",
		},
	}
}
