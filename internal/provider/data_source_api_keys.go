package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewAPIKeysDataSource)
}

var (
	_ datasource.DataSource              = (*apiKeysDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*apiKeysDataSource)(nil)
)

// NewAPIKeysDataSource returns the cycle_api_keys data source.
func NewAPIKeysDataSource() datasource.DataSource {
	return &apiKeysDataSource{}
}

type apiKeysDataSource struct {
	client *CycleClient
}

type apiKeysDataSourceModel struct {
	APIKeys []apiKeyDataSourceItemModel `tfsdk:"api_keys"`
}

type apiKeyDataSourceItemModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	RoleID types.String `tfsdk:"role_id"`
	IPs    types.List   `tfsdk:"ips"`
	HubID  types.String `tfsdk:"hub_id"`
	State  types.String `tfsdk:"state"`
}

func (d *apiKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_keys"
}

func (d *apiKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all Cycle hub API keys. The create-time `secret` is not exposed on this data source.",
		Attributes: map[string]schema.Attribute{
			"api_keys": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All API keys in the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the API key.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The user-defined name of the API key.",
						},
						"role_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the hub role this API key inherits permissions from.",
						},
						"ips": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "The allowlist of IP addresses that may use this API key. Empty means unrestricted.",
						},
						"hub_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the hub this API key belongs to.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the API key.",
						},
					},
				},
			},
		},
	}
}

func (d *apiKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *apiKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config apiKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys, err := fetchAllAPIKeys(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing API keys", err.Error())
		return
	}

	config.APIKeys = make([]apiKeyDataSourceItemModel, 0, len(keys))
	for _, key := range keys {
		item, d := apiKeyDataSourceItemFromAPI(ctx, key)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.APIKeys = append(config.APIKeys, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// fetchAllAPIKeys pages through GET /v1/api-keys and returns every API key
// in the hub.
func fetchAllAPIKeys(ctx context.Context, client *CycleClient) ([]cycle.ApiKey, error) {
	const pageSize = 100

	var all []cycle.ApiKey
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		listResp, err := client.Client.GetApiKeysWithResponse(ctx, &cycle.GetApiKeysParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing API keys", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
