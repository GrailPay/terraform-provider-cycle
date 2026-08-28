package provider

import (
	"context"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewStackBuildDataSource)
}

var (
	_ datasource.DataSource                     = (*stackBuildDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*stackBuildDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*stackBuildDataSource)(nil)
)

// NewStackBuildDataSource returns the cycle_stack_build data source.
func NewStackBuildDataSource() datasource.DataSource {
	return &stackBuildDataSource{}
}

type stackBuildDataSource struct {
	client *CycleClient
}

type stackBuildDataModel struct {
	ID          types.String `tfsdk:"id"`
	StackID     types.String `tfsdk:"stack_id"`
	Version     types.String `tfsdk:"version"`
	Description types.String `tfsdk:"description"`
	State       types.String `tfsdk:"state"`
	HubID       types.String `tfsdk:"hub_id"`
	GitType     types.String `tfsdk:"git_type"`
	GitValue    types.String `tfsdk:"git_value"`
	Variables   types.Map    `tfsdk:"variables"`
}

func (d *stackBuildDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack_build"
}

func (d *stackBuildDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle stack build by ID or by `about.version`. Exactly one of `id` or `version` must be set. Use this to reference an existing build without creating a new one.",
		Attributes:          stackBuildDataAttributes(true),
	}
}

func (d *stackBuildDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("version"),
		),
	}
}

func (d *stackBuildDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *stackBuildDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config stackBuildDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stackID := config.StackID.ValueString()
	var build cycle.StackBuild
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		getResp, err := d.client.Client.GetStackBuildWithResponse(ctx, stackID, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading stack build", err.Error())
			return
		}
		if getResp.StatusCode() == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Stack Build Not Found",
				fmt.Sprintf("No stack build with ID %q exists on stack %q.", config.ID.ValueString(), stackID),
			)
			return
		}
		if getResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "reading stack build", getResp.StatusCode(), getResp.JSONDefault)
			return
		}
		build = getResp.JSON200.Data
	} else {
		found, diags := findStackBuildByVersion(ctx, d.client, stackID, config.Version.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		build = *found
	}

	resp.Diagnostics.Append(stackBuildDataFromAPI(ctx, &config, build)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func stackBuildDataAttributes(lookup bool) map[string]schema.Attribute {
	id := schema.StringAttribute{
		MarkdownDescription: "The unique ID of the stack build.",
	}
	version := schema.StringAttribute{
		MarkdownDescription: "The version string from the build's `about.version` field.",
	}
	if lookup {
		id.Optional = true
		id.Computed = true
		id.MarkdownDescription += " Exactly one of `id` or `version` must be set."
		version.Optional = true
		version.Computed = true
		version.MarkdownDescription += " Exactly one of `id` or `version` must be set. The lookup fails if multiple non-deleted builds share that version."
	} else {
		id.Computed = true
		version.Computed = true
	}

	stackID := schema.StringAttribute{
		MarkdownDescription: "The ID of the stack this build belongs to.",
	}
	if lookup {
		stackID.Required = true
	} else {
		stackID.Computed = true
	}

	return map[string]schema.Attribute{
		"id":       id,
		"stack_id": stackID,
		"version":  version,
		"description": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The user-defined description of the build.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The current state of the build (for example `live`).",
		},
		"hub_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the hub this build belongs to.",
		},
		"git_type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The git ref type used for this build (`branch`, `tag`, or `commit`), if the build was created from git.",
		},
		"git_value": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The git branch, tag, or commit used for this build, if the build was created from git.",
		},
		"variables": schema.MapAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Build-time variables supplied when the build was created.",
		},
	}
}

func stackBuildDataFromAPI(ctx context.Context, model *stackBuildDataModel, build cycle.StackBuild) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(build.Id)
	model.StackID = types.StringValue(build.StackId)
	model.Version = types.StringValue(build.About.Version)
	model.Description = types.StringValue(build.About.Description)
	model.State = types.StringValue(string(build.State.Current))
	model.HubID = types.StringValue(build.HubId)
	model.GitType = types.StringNull()
	model.GitValue = types.StringNull()
	model.Variables = types.MapNull(types.StringType)

	if build.Instructions.Git != nil {
		model.GitType = types.StringValue(string(build.Instructions.Git.Type))
		model.GitValue = types.StringValue(build.Instructions.Git.Value)
	}
	if build.Instructions.Variables != nil {
		vars, d := types.MapValueFrom(ctx, types.StringType, *build.Instructions.Variables)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		model.Variables = vars
	}
	return diags
}

func findStackBuildByVersion(ctx context.Context, client *CycleClient, stackID, version string) (*cycle.StackBuild, diag.Diagnostics) {
	var diags diag.Diagnostics

	builds, err := fetchAllStackBuilds(ctx, client, stackID, "")
	if err != nil {
		diags.AddError("Error listing stack builds", err.Error())
		return nil, diags
	}

	var matches []cycle.StackBuild
	for _, build := range builds {
		if build.About.Version != version {
			continue
		}
		if build.State.Current == cycle.StackBuildStateCurrentDeleted || build.State.Current == cycle.StackBuildStateCurrentDeleting {
			continue
		}
		matches = append(matches, build)
	}

	switch len(matches) {
	case 0:
		diags.AddError(
			"Stack Build Not Found",
			fmt.Sprintf("No stack build with version %q exists on stack %q.", version, stackID),
		)
		return nil, diags
	case 1:
		return &matches[0], diags
	default:
		diags.AddError(
			"Multiple Stack Builds Found",
			fmt.Sprintf("Found %d stack builds with version %q on stack %q. Use the id attribute to select one unambiguously.", len(matches), version, stackID),
		)
		return nil, diags
	}
}

func fetchAllStackBuilds(ctx context.Context, client *CycleClient, stackID, state string) ([]cycle.StackBuild, error) {
	const pageSize = 100

	var all []cycle.StackBuild
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		params := &cycle.GetStackBuildsParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		}
		if state != "" {
			params.Filter = &struct {
				Search *string `json:"search,omitempty"`
				State  *string `json:"state,omitempty"`
			}{State: &state}
		}

		listResp, err := client.Client.GetStackBuildsWithResponse(ctx, stackID, params)
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing stack builds", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
