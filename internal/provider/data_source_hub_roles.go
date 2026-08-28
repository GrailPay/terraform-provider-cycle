package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewHubRolesDataSource)
}

var (
	_ datasource.DataSource              = (*hubRolesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*hubRolesDataSource)(nil)
)

// NewHubRolesDataSource returns the cycle_hub_roles data source.
func NewHubRolesDataSource() datasource.DataSource {
	return &hubRolesDataSource{}
}

type hubRolesDataSource struct {
	client *CycleClient
}

type hubRolesDataSourceModel struct {
	Roles []hubRolesDataSourceRoleModel `tfsdk:"roles"`
}

type hubRolesDataSourceRoleModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Identifier      types.String `tfsdk:"identifier"`
	Rank            types.Int64  `tfsdk:"rank"`
	Root            types.Bool   `tfsdk:"root"`
	AllCapabilities types.Bool   `tfsdk:"all_capabilities"`
	Capabilities    types.Set    `tfsdk:"capabilities"`
	DefaultRole     types.String `tfsdk:"default_role"`
}

func (d *hubRolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub_roles"
}

func (d *hubRolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves all roles defined on the current Cycle hub, including the built-in default roles and any custom roles (`/v1/hubs/current/roles`).",
		Attributes: map[string]schema.Attribute{
			"roles": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of roles on the hub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the role.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The human-readable name of the role.",
						},
						"identifier": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The slug identifier of the role.",
						},
						"rank": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "The hierarchy rank of the role (0-10).",
						},
						"root": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this role has full moderation control over all other roles.",
						},
						"all_capabilities": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this role has every platform capability.",
						},
						"capabilities": schema.SetAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "The specific capability identifiers granted to this role. Empty when `all_capabilities` is `true`.",
						},
						"default_role": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The identifier of the built-in default role this role was created from, or null for fully custom roles.",
						},
					},
				},
			},
		},
	}
}

func (d *hubRolesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *hubRolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		return
	}

	const pageSize = 100
	var roles []cycle.Role
	for page := float32(1); ; page++ {
		size := float32(pageSize)
		number := page
		apiResp, err := d.client.Client.GetRolesWithResponse(ctx, &cycle.GetRolesParams{
			Page: &cycle.PageParam{Number: &number, Size: &size},
		})
		if err != nil {
			resp.Diagnostics.AddError("Error listing hub roles", err.Error())
			return
		}
		if apiResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "listing hub roles", apiResp.StatusCode(), apiResp.JSONDefault)
			return
		}

		roles = append(roles, apiResp.JSON200.Data...)
		if len(apiResp.JSON200.Data) < pageSize {
			break
		}
	}

	state := hubRolesDataSourceModel{
		Roles: make([]hubRolesDataSourceRoleModel, 0, len(roles)),
	}
	for _, role := range roles {
		specific := make([]string, 0, len(role.Capabilities.Specific))
		for _, c := range role.Capabilities.Specific {
			specific = append(specific, string(c))
		}
		specificSet, diags := types.SetValueFrom(ctx, types.StringType, specific)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		m := hubRolesDataSourceRoleModel{
			ID:              types.StringValue(role.Id),
			Identifier:      types.StringValue(role.Identifier),
			Rank:            types.Int64Value(int64(role.Rank)),
			Root:            types.BoolValue(role.Root),
			AllCapabilities: types.BoolValue(role.Capabilities.All),
			Capabilities:    specificSet,
			Name:            types.StringNull(),
			DefaultRole:     types.StringNull(),
		}
		if role.Name != nil {
			m.Name = types.StringValue(*role.Name)
		}
		if role.Default != nil {
			m.DefaultRole = types.StringValue(*role.Default)
		}
		state.Roles = append(state.Roles, m)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
