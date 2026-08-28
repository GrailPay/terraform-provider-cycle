package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewHubMembersDataSource)
}

var (
	_ datasource.DataSource              = (*hubMembersDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*hubMembersDataSource)(nil)
)

// NewHubMembersDataSource returns the cycle_hub_members data source.
func NewHubMembersDataSource() datasource.DataSource {
	return &hubMembersDataSource{}
}

type hubMembersDataSource struct {
	client *CycleClient
}

type hubMembersDataSourceModel struct {
	Members []hubMembersDataSourceMemberModel `tfsdk:"members"`
}

type hubMembersDataSourceMemberModel struct {
	ID        types.String `tfsdk:"id"`
	AccountID types.String `tfsdk:"account_id"`
	Email     types.String `tfsdk:"email"`
	RoleID    types.String `tfsdk:"role_id"`
	State     types.String `tfsdk:"state"`
}

func (d *hubMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub_members"
}

func (d *hubMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves all members of the current Cycle hub, with their account email and assigned role (`/v1/hubs/current/members`).",
		Attributes: map[string]schema.Attribute{
			"members": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of hub members.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the hub membership.",
						},
						"account_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the member's Cycle account.",
						},
						"email": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The email address of the member's account.",
						},
						"role_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the hub role assigned to this member.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the membership (e.g. `accepted`).",
						},
					},
				},
			},
		},
	}
}

func (d *hubMembersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *hubMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		return
	}

	const pageSize = 100
	include := []cycle.GetHubMembersParamsInclude{cycle.GetHubMembersParamsIncludeAccounts}

	var members []cycle.HubMembership
	accounts := map[string]cycle.PublicAccount{}
	for page := float32(1); ; page++ {
		size := float32(pageSize)
		number := page
		apiResp, err := d.client.Client.GetHubMembersWithResponse(ctx, &cycle.GetHubMembersParams{
			Include: &include,
			Page:    &cycle.PageParam{Number: &number, Size: &size},
		})
		if err != nil {
			resp.Diagnostics.AddError("Error listing hub members", err.Error())
			return
		}
		if apiResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "listing hub members", apiResp.StatusCode(), apiResp.JSONDefault)
			return
		}

		members = append(members, apiResp.JSON200.Data...)
		if inc := apiResp.JSON200.Includes; inc != nil && inc.Accounts != nil {
			for id, account := range *inc.Accounts {
				accounts[id] = account
			}
		}
		if len(apiResp.JSON200.Data) < pageSize {
			break
		}
	}

	state := hubMembersDataSourceModel{
		Members: make([]hubMembersDataSourceMemberModel, 0, len(members)),
	}
	for _, membership := range members {
		if membership.State.Current == cycle.MembershipStateCurrentDeleted {
			continue
		}

		m := hubMembersDataSourceMemberModel{
			ID:        types.StringValue(membership.Id),
			RoleID:    types.StringValue(membership.RoleId),
			State:     types.StringValue(string(membership.State.Current)),
			AccountID: types.StringNull(),
			Email:     types.StringNull(),
		}
		if membership.AccountId != nil {
			m.AccountID = types.StringValue(*membership.AccountId)
			if account, ok := accounts[*membership.AccountId]; ok {
				m.Email = types.StringValue(account.Email.Address)
			}
		}
		state.Members = append(state.Members, m)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
