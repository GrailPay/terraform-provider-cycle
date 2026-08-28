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
	RegisterDataSource(NewIntegrationDataSource)
}

var (
	_ datasource.DataSource              = (*integrationDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*integrationDataSource)(nil)
)

// NewIntegrationDataSource returns the cycle_integration data source.
func NewIntegrationDataSource() datasource.DataSource {
	return &integrationDataSource{}
}

type integrationDataSource struct {
	client *CycleClient
}

func (d *integrationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (d *integrationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle hub integration by ID.",
		Attributes:          integrationDataSourceAttributes(true),
	}
}

func (d *integrationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *integrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config integrationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Client.GetIntegrationWithResponse(ctx, config.ID.ValueString(), &cycle.GetIntegrationParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading integration", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Integration Not Found",
			fmt.Sprintf("No integration with ID %q exists in this hub.", config.ID.ValueString()),
		)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading integration", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(integrationModelFromAPI(ctx, &config, getResp.JSON200.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func integrationDataSourceAuthAttributes() map[string]schema.Attribute {
	field := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Computed:            true,
			Sensitive:           true,
			MarkdownDescription: desc,
		}
	}

	return map[string]schema.Attribute{
		"api_key":         field("API key for accessing the integration."),
		"base64_config":   field("Base64-encoded configuration for the integration."),
		"client_id":       field("Client ID for the integration."),
		"key_id":          field("Key ID for accessing the integration."),
		"namespace":       field("The namespace associated with the integration."),
		"region":          field("The region associated with the integration."),
		"secret":          field("Secret for accessing the integration."),
		"subscription_id": field("Subscription ID for the integration."),
	}
}

func integrationDataSourceAttributes(idRequired bool) map[string]schema.Attribute {
	id := schema.StringAttribute{
		MarkdownDescription: "The unique ID of the integration.",
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
			MarkdownDescription: "The user-defined name of the integration.",
		},
		"identifier": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The human-readable slugged identifier of the integration.",
		},
		"vendor": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The vendor this integration is associated with.",
		},
		"auth": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Vendor-specific authentication credentials. Sensitive fields are often omitted by the API after create.",
			Attributes:          integrationDataSourceAuthAttributes(),
		},
		"extra": schema.MapAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			Sensitive:           true,
			MarkdownDescription: "Additional vendor-specific key-value pairs.",
		},
		"hub_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the hub this integration belongs to.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The current state of the integration.",
		},
	}
}
