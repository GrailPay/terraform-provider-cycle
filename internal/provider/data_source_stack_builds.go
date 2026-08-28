package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewStackBuildsDataSource)
}

var (
	_ datasource.DataSource              = (*stackBuildsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*stackBuildsDataSource)(nil)
)

// NewStackBuildsDataSource returns the cycle_stack_builds data source.
func NewStackBuildsDataSource() datasource.DataSource {
	return &stackBuildsDataSource{}
}

type stackBuildsDataSource struct {
	client *CycleClient
}

type stackBuildsDataSourceModel struct {
	StackID types.String          `tfsdk:"stack_id"`
	State   types.String          `tfsdk:"state"`
	Builds  []stackBuildDataModel `tfsdk:"builds"`
}

func (d *stackBuildsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack_builds"
}

func (d *stackBuildsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Cycle stack builds for a stack. Deleted builds are omitted. Set `state` to `live` to find builds that can be deployed.",
		Attributes: map[string]schema.Attribute{
			"stack_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the stack whose builds should be listed.",
			},
			"state": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "When set, only builds in this state are returned (for example `live`).",
			},
			"builds": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The stack builds matching the filter.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: stackBuildDataAttributes(false),
				},
			},
		},
	}
}

func (d *stackBuildsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *stackBuildsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config stackBuildsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	builds, err := fetchAllStackBuilds(ctx, d.client, config.StackID.ValueString(), config.State.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing stack builds", err.Error())
		return
	}

	config.Builds = make([]stackBuildDataModel, 0, len(builds))
	for _, build := range builds {
		if build.State.Current == cycle.StackBuildStateCurrentDeleted || build.State.Current == cycle.StackBuildStateCurrentDeleting {
			continue
		}
		var item stackBuildDataModel
		resp.Diagnostics.Append(stackBuildDataFromAPI(ctx, &item, build)...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.Builds = append(config.Builds, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
