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
	RegisterDataSource(NewScopedVariableDataSource)
}

var (
	_ datasource.DataSource                     = (*scopedVariableDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*scopedVariableDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*scopedVariableDataSource)(nil)
)

// NewScopedVariableDataSource returns the cycle_scoped_variable data source.
func NewScopedVariableDataSource() datasource.DataSource {
	return &scopedVariableDataSource{}
}

type scopedVariableDataSource struct {
	client *CycleClient
}

type scopedVariableDataModel struct {
	ID            types.String               `tfsdk:"id"`
	EnvironmentID types.String               `tfsdk:"environment_id"`
	Identifier    types.String               `tfsdk:"identifier"`
	Value         types.String               `tfsdk:"value"`
	Secret        *scopedVariableSecretModel `tfsdk:"secret"`
	Scope         *scopedVariableScopeModel  `tfsdk:"scope"`
	Access        *scopedVariableAccessModel `tfsdk:"access"`
	HubID         types.String               `tfsdk:"hub_id"`
	State         types.String               `tfsdk:"state"`
}

func (d *scopedVariableDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scoped_variable"
}

func (d *scopedVariableDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle scoped variable in an environment by ID or identifier. Exactly one of `id` or `identifier` must be set.",
		Attributes:          scopedVariableDataAttributes(true),
	}
}

func (d *scopedVariableDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("identifier"),
		),
	}
}

func (d *scopedVariableDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *scopedVariableDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config scopedVariableDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	environmentID := config.EnvironmentID.ValueString()
	var sv cycle.ScopedVariable
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		getResp, err := d.client.Client.GetScopedVariableWithResponse(ctx, environmentID, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading scoped variable", err.Error())
			return
		}
		if getResp.StatusCode() == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Scoped Variable Not Found",
				fmt.Sprintf("No scoped variable with ID %q exists in environment %q.", config.ID.ValueString(), environmentID),
			)
			return
		}
		if getResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "reading scoped variable", getResp.StatusCode(), getResp.JSONDefault)
			return
		}
		sv = getResp.JSON200.Data
	} else {
		found, diags := findScopedVariableByIdentifier(ctx, d.client, environmentID, config.Identifier.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		sv = *found
	}

	resp.Diagnostics.Append(scopedVariableDataFromAPI(ctx, &config, sv)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func scopedVariableDataAttributes(lookup bool) map[string]schema.Attribute {
	id := schema.StringAttribute{
		MarkdownDescription: "The unique ID of the scoped variable.",
	}
	identifier := schema.StringAttribute{
		MarkdownDescription: "The identifier of the scoped variable, similar to the key of an environment variable.",
	}
	if lookup {
		id.Optional = true
		id.Computed = true
		id.MarkdownDescription += " Exactly one of `id` or `identifier` must be set."
		identifier.Optional = true
		identifier.Computed = true
		identifier.MarkdownDescription += " Exactly one of `id` or `identifier` must be set. Identifiers are not guaranteed unique; the lookup fails if multiple live variables match."
	} else {
		id.Computed = true
		identifier.Computed = true
	}

	environmentID := schema.StringAttribute{
		MarkdownDescription: "The ID of the environment this scoped variable belongs to.",
	}
	if lookup {
		environmentID.Required = true
	} else {
		environmentID.Computed = true
	}

	return map[string]schema.Attribute{
		"id":             id,
		"environment_id": environmentID,
		"identifier":     identifier,
		"value": schema.StringAttribute{
			Computed:            true,
			Sensitive:           true,
			MarkdownDescription: "The raw value of the scoped variable, when the API returns a raw source.",
		},
		"secret": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Present when the variable is marked as a secret on the Cycle dashboard.",
			Attributes: map[string]schema.Attribute{
				"hint": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "A user-specified hint for portal-encrypted values.",
				},
				"iv": schema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "The IV hex associated with portal-side encryption, if any.",
				},
			},
		},
		"scope": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Which containers in the environment the variable is assigned to.",
			Attributes: map[string]schema.Attribute{
				"global": schema.BoolAttribute{
					Computed:            true,
					MarkdownDescription: "Whether the variable is assigned to all current and future containers in the environment.",
				},
				"container_ids": schema.ListAttribute{
					ElementType:         types.StringType,
					Computed:            true,
					MarkdownDescription: "Container IDs that have access to this scoped variable.",
				},
				"container_identifiers": schema.ListAttribute{
					ElementType:         types.StringType,
					Computed:            true,
					MarkdownDescription: "Container identifiers that have access to this scoped variable.",
				},
			},
		},
		"access": schema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "How the scoped variable is exposed to containers.",
			Attributes: map[string]schema.Attribute{
				"env_variable": schema.SingleNestedAttribute{
					Computed:            true,
					MarkdownDescription: "Present when the variable is exposed as a container environment variable.",
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The environment variable name set on the target container.",
						},
					},
				},
				"internal_api": schema.SingleNestedAttribute{
					Computed:            true,
					MarkdownDescription: "Present when the variable is exposed over Cycle's internal API.",
					Attributes: map[string]schema.Attribute{
						"duration": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "How long the internal API serves the variable after runtime start.",
						},
					},
				},
				"file": schema.SingleNestedAttribute{
					Computed:            true,
					MarkdownDescription: "Present when the variable is mounted as a file inside the container.",
					Attributes: map[string]schema.Attribute{
						"path": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The path the file is mounted to inside the container.",
						},
						"decode": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether Cycle base64-decodes the value before writing the file.",
						},
					},
				},
			},
		},
		"hub_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the hub this scoped variable belongs to.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The current state of the scoped variable.",
		},
	}
}

func scopedVariableDataFromAPI(ctx context.Context, model *scopedVariableDataModel, sv cycle.ScopedVariable) diag.Diagnostics {
	var diags diag.Diagnostics
	var res scopedVariableResourceModel
	scopedVariableModelFromAPI(ctx, &res, sv, &diags)
	if diags.HasError() {
		return diags
	}

	model.ID = res.ID
	model.EnvironmentID = res.EnvironmentID
	model.Identifier = res.Identifier
	model.Value = res.Value
	model.Secret = res.Secret
	model.Scope = res.Scope
	model.Access = res.Access
	model.HubID = res.HubID
	model.State = types.StringValue(string(sv.State.Current))
	return diags
}

func findScopedVariableByIdentifier(ctx context.Context, client *CycleClient, environmentID, identifier string) (*cycle.ScopedVariable, diag.Diagnostics) {
	var diags diag.Diagnostics

	variables, err := fetchAllScopedVariables(ctx, client, environmentID, &identifier)
	if err != nil {
		diags.AddError("Error listing scoped variables", err.Error())
		return nil, diags
	}

	var live []cycle.ScopedVariable
	for _, sv := range variables {
		if sv.Identifier != identifier {
			continue
		}
		if sv.State.Current == cycle.ScopedVariableStateCurrentDeleted || sv.State.Current == cycle.ScopedVariableStateCurrentDeleting {
			continue
		}
		live = append(live, sv)
	}

	switch len(live) {
	case 0:
		diags.AddError(
			"Scoped Variable Not Found",
			fmt.Sprintf("No scoped variable with identifier %q exists in environment %q.", identifier, environmentID),
		)
		return nil, diags
	case 1:
		return &live[0], diags
	default:
		diags.AddError(
			"Multiple Scoped Variables Found",
			fmt.Sprintf("Found %d scoped variables with identifier %q in environment %q. Use the id attribute to select one unambiguously.", len(live), identifier, environmentID),
		)
		return nil, diags
	}
}

func fetchAllScopedVariables(ctx context.Context, client *CycleClient, environmentID string, identifier *string) ([]cycle.ScopedVariable, error) {
	const pageSize = 100

	var all []cycle.ScopedVariable
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		params := &cycle.GetScopedVariablesParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		}
		if identifier != nil {
			params.Filter = &struct {
				Container  *string `json:"container,omitempty"`
				Identifier *string `json:"identifier,omitempty"`
				Search     *string `json:"search,omitempty"`
				State      *string `json:"state,omitempty"`
			}{Identifier: identifier}
		}

		listResp, err := client.Client.GetScopedVariablesWithResponse(ctx, environmentID, params)
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing scoped variables", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
