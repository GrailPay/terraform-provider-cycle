package provider

import (
	"context"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewEnvironmentDataSource)
}

var (
	_ datasource.DataSource                     = (*environmentDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*environmentDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*environmentDataSource)(nil)
)

// NewEnvironmentDataSource returns the cycle_environment data source.
func NewEnvironmentDataSource() datasource.DataSource {
	return &environmentDataSource{}
}

type environmentDataSource struct {
	client *CycleClient
}

type environmentDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Identifier       types.String `tfsdk:"identifier"`
	Cluster          types.String `tfsdk:"cluster"`
	Description      types.String `tfsdk:"description"`
	LegacyNetworking types.Bool   `tfsdk:"legacy_networking"`
	HubID            types.String `tfsdk:"hub_id"`
	State            types.String `tfsdk:"state"`
}

func (d *environmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *environmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle environment by ID, name, or identifier.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique ID of the environment. Exactly one of `id`, `name`, or `identifier` must be set.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The user-defined name of the environment. Exactly one of `id`, `name`, or `identifier` must be set.",
			},
			"identifier": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The human-readable slugged identifier of the environment. Exactly one of `id`, `name`, or `identifier` must be set.",
			},
			"cluster": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The identifier of the cluster this environment is deployed into.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The custom description of the environment.",
			},
			"legacy_networking": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether legacy networking mode is enabled on this environment.",
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this environment belongs to.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the environment.",
			},
		},
	}
}

func (d *environmentDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
			path.MatchRoot("identifier"),
		),
	}
}

func (d *environmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *environmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config environmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var env cycle.Environment
	if !config.ID.IsNull() {
		getResp, err := d.client.Client.GetEnvironmentWithResponse(ctx, config.ID.ValueString(), &cycle.GetEnvironmentParams{})
		if err != nil {
			resp.Diagnostics.AddError("Error reading environment", err.Error())
			return
		}
		if getResp.StatusCode() == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Environment Not Found",
				fmt.Sprintf("No environment with ID %q exists in this hub.", config.ID.ValueString()),
			)
			return
		}
		if getResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "reading environment", getResp.StatusCode(), getResp.JSONDefault)
			return
		}
		env = getResp.JSON200.Data
	} else {
		environments, err := fetchAllEnvironments(ctx, d.client)
		if err != nil {
			resp.Diagnostics.AddError("Error listing environments", err.Error())
			return
		}

		attribute, wanted := "name", config.Name.ValueString()
		match := func(e cycle.Environment) bool { return e.Name == wanted }
		if !config.Identifier.IsNull() {
			attribute, wanted = "identifier", config.Identifier.ValueString()
			match = func(e cycle.Environment) bool { return e.Identifier == wanted }
		}

		var matches []cycle.Environment
		for _, e := range environments {
			if match(e) {
				matches = append(matches, e)
			}
		}
		switch len(matches) {
		case 0:
			resp.Diagnostics.AddError(
				"Environment Not Found",
				fmt.Sprintf("No environment with %s %q exists in this hub.", attribute, wanted),
			)
			return
		case 1:
			env = matches[0]
		default:
			resp.Diagnostics.AddError(
				"Multiple Environments Found",
				fmt.Sprintf("Found %d environments with %s %q. Use the id attribute to select one unambiguously.", len(matches), attribute, wanted),
			)
			return
		}
	}

	config.ID = types.StringValue(env.Id)
	config.Name = types.StringValue(env.Name)
	config.Identifier = types.StringValue(env.Identifier)
	config.Cluster = types.StringValue(env.Cluster)
	config.Description = types.StringValue(env.About.Description)
	config.LegacyNetworking = types.BoolValue(env.Features.LegacyNetworking)
	config.HubID = types.StringValue(env.HubId)
	config.State = types.StringValue(string(env.State.Current))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
