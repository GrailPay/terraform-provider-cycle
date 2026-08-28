package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewScopedVariablesDataSource)
}

var (
	_ datasource.DataSource              = (*scopedVariablesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*scopedVariablesDataSource)(nil)
)

// NewScopedVariablesDataSource returns the cycle_scoped_variables data source.
func NewScopedVariablesDataSource() datasource.DataSource {
	return &scopedVariablesDataSource{}
}

type scopedVariablesDataSource struct {
	client *CycleClient
}

type scopedVariablesDataSourceModel struct {
	EnvironmentID types.String              `tfsdk:"environment_id"`
	Variables     []scopedVariableDataModel `tfsdk:"variables"`
}

func (d *scopedVariablesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scoped_variables"
}

func (d *scopedVariablesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Cycle scoped variables in an environment. Deleted variables are omitted. Values are marked sensitive.",
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the environment whose scoped variables should be listed.",
			},
			"variables": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The scoped variables in the environment.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: scopedVariableDataAttributes(false),
				},
			},
		},
	}
}

func (d *scopedVariablesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *scopedVariablesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config scopedVariablesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	variables, err := fetchAllScopedVariables(ctx, d.client, config.EnvironmentID.ValueString(), nil)
	if err != nil {
		resp.Diagnostics.AddError("Error listing scoped variables", err.Error())
		return
	}

	config.Variables = make([]scopedVariableDataModel, 0, len(variables))
	for _, sv := range variables {
		if sv.State.Current == cycle.ScopedVariableStateCurrentDeleted || sv.State.Current == cycle.ScopedVariableStateCurrentDeleting {
			continue
		}
		var item scopedVariableDataModel
		resp.Diagnostics.Append(scopedVariableDataFromAPI(ctx, &item, sv)...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.Variables = append(config.Variables, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
