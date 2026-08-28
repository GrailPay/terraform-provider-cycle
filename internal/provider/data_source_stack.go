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
	RegisterDataSource(NewStackDataSource)
}

var (
	_ datasource.DataSource                     = (*stackDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*stackDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*stackDataSource)(nil)
)

// NewStackDataSource returns the cycle_stack data source.
func NewStackDataSource() datasource.DataSource {
	return &stackDataSource{}
}

type stackDataSource struct {
	client *CycleClient
}

type stackDataSourceModel struct {
	ID         types.String      `tfsdk:"id"`
	Name       types.String      `tfsdk:"name"`
	Identifier types.String      `tfsdk:"identifier"`
	Variables  types.Map         `tfsdk:"variables"`
	Source     *stackSourceModel `tfsdk:"source"`
	HubID      types.String      `tfsdk:"hub_id"`
	State      types.String      `tfsdk:"state"`
}

func (d *stackDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack"
}

func (d *stackDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle stack by ID or identifier. Exactly one of `id` or `identifier` must be set.",
		Attributes:          stackDataSourceAttributes(true),
	}
}

func (d *stackDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("identifier"),
		),
	}
}

func (d *stackDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *stackDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config stackDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var stack cycle.Stack
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		getResp, err := d.client.Client.GetStackWithResponse(ctx, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading stack", err.Error())
			return
		}
		if getResp.StatusCode() == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Stack Not Found",
				fmt.Sprintf("No stack with ID %q exists in this hub.", config.ID.ValueString()),
			)
			return
		}
		if getResp.JSON200 == nil {
			addAPIError(&resp.Diagnostics, "reading stack", getResp.StatusCode(), getResp.JSONDefault)
			return
		}
		stack = getResp.JSON200.Data
	} else {
		found, diags := findStackByIdentifier(ctx, d.client, config.Identifier.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		stack = *found
	}

	resp.Diagnostics.Append(stackDataSourceModelFromAPI(ctx, &config, stack)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func stackDataSourceAttributes(lookup bool) map[string]schema.Attribute {
	id := schema.StringAttribute{
		MarkdownDescription: "The unique ID of the stack.",
	}
	identifier := schema.StringAttribute{
		MarkdownDescription: "The human-readable slugged identifier of the stack.",
	}
	if lookup {
		id.Optional = true
		id.Computed = true
		id.MarkdownDescription += " Exactly one of `id` or `identifier` must be set."
		identifier.Optional = true
		identifier.Computed = true
		identifier.MarkdownDescription += " Exactly one of `id` or `identifier` must be set."
	} else {
		id.Computed = true
		identifier.Computed = true
	}

	return map[string]schema.Attribute{
		"id": id,
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The user-defined name of the stack.",
		},
		"identifier": identifier,
		"variables": schema.MapAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Default variable values configured on the stack.",
		},
		"source": stackSourceSchema(true),
		"hub_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the hub this stack belongs to.",
		},
		"state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The current state of the stack.",
		},
	}
}

func stackDataSourceModelFromAPI(ctx context.Context, model *stackDataSourceModel, stack cycle.Stack) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(stack.Id)
	model.Name = types.StringValue(stack.Name)
	model.Identifier = types.StringValue(stack.Identifier)
	model.HubID = types.StringValue(stack.HubId)
	model.State = types.StringValue(string(stack.State.Current))

	if stack.Variables == nil {
		model.Variables = types.MapNull(types.StringType)
	} else {
		vars, d := types.MapValueFrom(ctx, types.StringType, *stack.Variables)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		model.Variables = vars
	}

	source, d := flattenStackSource(stack.Source, nil)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	model.Source = source
	return diags
}

func findStackByIdentifier(ctx context.Context, client *CycleClient, identifier string) (*cycle.Stack, diag.Diagnostics) {
	var diags diag.Diagnostics

	const pageSize = 100
	var matches []cycle.Stack
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		ident := identifier
		listResp, err := client.Client.GetStacksWithResponse(ctx, &cycle.GetStacksParams{
			Filter: &struct {
				Identifier *string `json:"identifier,omitempty"`
				Search     *string `json:"search,omitempty"`
				State      *string `json:"state,omitempty"`
			}{Identifier: &ident},
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		})
		if err != nil {
			diags.AddError("Error listing stacks", err.Error())
			return nil, diags
		}
		if listResp.JSON200 == nil {
			addAPIError(&diags, "listing stacks", listResp.StatusCode(), listResp.JSONDefault)
			return nil, diags
		}

		matches = append(matches, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			break
		}
	}

	switch len(matches) {
	case 0:
		diags.AddError(
			"Stack Not Found",
			fmt.Sprintf("No stack with identifier %q exists in this hub.", identifier),
		)
		return nil, diags
	case 1:
		return &matches[0], diags
	default:
		diags.AddError(
			"Multiple Stacks Found",
			fmt.Sprintf("Found %d stacks with identifier %q. Use the id attribute to select one unambiguously.", len(matches), identifier),
		)
		return nil, diags
	}
}
