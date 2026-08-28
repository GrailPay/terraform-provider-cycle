package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewHubDataSource)
}

var (
	_ datasource.DataSource              = (*hubDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*hubDataSource)(nil)
)

// NewHubDataSource returns the cycle_hub data source.
func NewHubDataSource() datasource.DataSource {
	return &hubDataSource{}
}

type hubDataSource struct {
	client *CycleClient
}

type hubDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Identifier types.String `tfsdk:"identifier"`
	State      types.String `tfsdk:"state"`
}

func (d *hubDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub"
}

func (d *hubDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about the Cycle hub the provider is configured for (`/v1/hubs/current`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the hub.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name of the hub.",
			},
			"identifier": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The human-readable slug identifier of the hub.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the hub (e.g. `live`).",
			},
		},
	}
}

func (d *hubDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *hubDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		return
	}

	apiResp, err := d.client.Client.GetHubWithResponse(ctx, &cycle.GetHubParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading hub", err.Error())
		return
	}
	if apiResp.JSON200 == nil || apiResp.JSON200.Data == nil {
		addAPIError(&resp.Diagnostics, "reading hub", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	hub := apiResp.JSON200.Data
	state := hubDataSourceModel{
		ID:         types.StringValue(hub.Id),
		Name:       types.StringValue(hub.Name),
		Identifier: types.StringValue(hub.Identifier),
		State:      types.StringValue(string(hub.State.Current)),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
