package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewProviderLocationsDataSource)
}

var (
	_ datasource.DataSource              = (*providerLocationsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*providerLocationsDataSource)(nil)
)

// NewProviderLocationsDataSource returns the cycle_provider_locations data source.
func NewProviderLocationsDataSource() datasource.DataSource {
	return &providerLocationsDataSource{}
}

type providerLocationsDataSource struct {
	client *CycleClient
}

type providerLocationsDataSourceModel struct {
	IntegrationID types.String                           `tfsdk:"integration_id"`
	Locations     []providerLocationsDataSourceItemModel `tfsdk:"locations"`
}

type providerLocationsDataSourceItemModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Abbreviation      types.String `tfsdk:"abbreviation"`
	Compatible        types.Bool   `tfsdk:"compatible"`
	ProviderCode      types.String `tfsdk:"provider_code"`
	ProviderLocation  types.String `tfsdk:"provider_location"`
	AvailabilityZones types.List   `tfsdk:"availability_zones"`
}

func (d *providerLocationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider_locations"
}

func (d *providerLocationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists locations available from a Cycle provider integration. Use this to choose a `location_id` for `cycle_server`.",
		Attributes: map[string]schema.Attribute{
			"integration_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The provider integration ID to list locations for.",
			},
			"locations": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Locations advertised by the provider integration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The location ID.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The location name.",
						},
						"abbreviation": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "A short abbreviation for the location.",
						},
						"compatible": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether Cycle currently supports this location.",
						},
						"provider_code": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The location code returned by the provider.",
						},
						"provider_location": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The location name returned by the provider.",
						},
						"availability_zones": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Availability zones at this location, if the provider reports them.",
						},
					},
				},
			},
		},
	}
}

func (d *providerLocationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *providerLocationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config providerLocationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	locations, err := fetchAllProviderLocations(ctx, d.client, config.IntegrationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing provider locations", err.Error())
		return
	}

	config.Locations = make([]providerLocationsDataSourceItemModel, 0, len(locations))
	for _, loc := range locations {
		zones := []string{}
		if loc.Provider.AvailabilityZones != nil {
			zones = *loc.Provider.AvailabilityZones
		}
		zoneList, diags := types.ListValueFrom(ctx, types.StringType, zones)
		resp.Diagnostics.Append(diags...)

		config.Locations = append(config.Locations, providerLocationsDataSourceItemModel{
			ID:                types.StringValue(loc.Id),
			Name:              types.StringValue(loc.Name),
			Abbreviation:      types.StringValue(loc.Abbreviation),
			Compatible:        types.BoolValue(loc.Compatible),
			ProviderCode:      types.StringValue(loc.Provider.Code),
			ProviderLocation:  types.StringValue(loc.Provider.Location),
			AvailabilityZones: zoneList,
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func fetchAllProviderLocations(ctx context.Context, client *CycleClient, integrationID string) ([]cycle.ProviderLocation, error) {
	const pageSize = 100

	var all []cycle.ProviderLocation
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetProviderLocationsWithResponse(ctx, integrationID, &cycle.GetProviderLocationsParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing provider locations", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
