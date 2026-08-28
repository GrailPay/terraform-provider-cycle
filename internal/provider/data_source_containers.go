package provider

import (
	"context"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewContainersDataSource)
}

var (
	_ datasource.DataSource              = (*containersDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*containersDataSource)(nil)
)

// NewContainersDataSource returns the cycle_containers data source.
func NewContainersDataSource() datasource.DataSource {
	return &containersDataSource{}
}

type containersDataSource struct {
	client *CycleClient
}

type containersDataSourceModel struct {
	EnvironmentID types.String                    `tfsdk:"environment_id"`
	Containers    []containersDataSourceItemModel `tfsdk:"containers"`
}

type containersDataSourceItemModel struct {
	ID            types.String         `tfsdk:"id"`
	Name          types.String         `tfsdk:"name"`
	Identifier    types.String         `tfsdk:"identifier"`
	EnvironmentID types.String         `tfsdk:"environment_id"`
	ImageID       types.String         `tfsdk:"image_id"`
	Stateful      types.Bool           `tfsdk:"stateful"`
	State         types.String         `tfsdk:"state"`
	Instances     types.Int64          `tfsdk:"instances"`
	HubID         types.String         `tfsdk:"hub_id"`
	Config        jsontypes.Normalized `tfsdk:"config"`
}

func (d *containersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_containers"
}

func (d *containersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Cycle containers in the hub, optionally filtered to a single environment.",
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "When set, only containers belonging to this environment ID are returned.",
			},
			"containers": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of containers.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the container.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The user-defined name of the container.",
						},
						"identifier": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The human-readable slugged identifier of the container.",
						},
						"environment_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the environment this container is deployed into.",
						},
						"image_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the image used to create this container.",
						},
						"stateful": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this container is stateful.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the container.",
						},
						"instances": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "The number of instances for this container.",
						},
						"hub_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the hub this container belongs to.",
						},
						"config": schema.StringAttribute{
							Computed:            true,
							CustomType:          jsontypes.NormalizedType{},
							MarkdownDescription: "The container configuration as normalized JSON.",
						},
					},
				},
			},
		},
	}
}

func (d *containersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *containersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config containersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var environmentID *string
	if !config.EnvironmentID.IsNull() && !config.EnvironmentID.IsUnknown() && config.EnvironmentID.ValueString() != "" {
		value := config.EnvironmentID.ValueString()
		environmentID = &value
	}

	containers, err := fetchAllContainers(ctx, d.client, environmentID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing containers", err.Error())
		return
	}

	config.Containers = make([]containersDataSourceItemModel, 0, len(containers))
	for _, container := range containers {
		item, diags := containersDataSourceItemFromAPI(container)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.Containers = append(config.Containers, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func containersDataSourceItemFromAPI(container cycle.Container) (containersDataSourceItemModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	item := containersDataSourceItemModel{
		ID:            types.StringValue(container.Id),
		Name:          types.StringValue(container.Name),
		EnvironmentID: types.StringValue(container.Environment.Id),
		Stateful:      types.BoolValue(container.Stateful),
		State:         types.StringValue(string(container.State.Current)),
		Instances:     types.Int64Value(int64(container.Instances)),
		HubID:         types.StringValue(container.HubId),
	}
	if container.Identifier != "" {
		item.Identifier = types.StringValue(container.Identifier)
	} else {
		item.Identifier = types.StringNull()
	}
	if container.Image.Id != nil && *container.Image.Id != "" {
		item.ImageID = types.StringValue(*container.Image.Id)
	} else {
		item.ImageID = types.StringNull()
	}

	cfg, d := containerConfigToValue(&container.Config)
	diags.Append(d...)
	item.Config = cfg
	return item, diags
}

// fetchAllContainers pages through GET /v1/containers and returns every
// container in the hub, optionally filtered by environment ID.
func fetchAllContainers(ctx context.Context, client *CycleClient, environmentID *string) ([]cycle.Container, error) {
	const pageSize = 100

	var all []cycle.Container
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		params := &cycle.GetContainersParams{
			Page: &cycle.PageParam{
				Number: &number,
				Size:   &size,
			},
		}
		if environmentID != nil && *environmentID != "" {
			envID := *environmentID
			params.Filter = &struct {
				Creator            *string         `json:"creator,omitempty"`
				Deployment         *string         `json:"deployment,omitempty"`
				DeploymentStrategy *string         `json:"deployment_strategy,omitempty"`
				Deprecated         *string         `json:"deprecated,omitempty"`
				Environment        *string         `json:"environment,omitempty"`
				Identifier         *string         `json:"identifier,omitempty"`
				Image              *string         `json:"image,omitempty"`
				PublicNetwork      *string         `json:"public_network,omitempty"`
				RangeEnd           *cycle.DateTime `json:"range-end,omitempty"`
				RangeStart         *cycle.DateTime `json:"range-start,omitempty"`
				Search             *string         `json:"search,omitempty"`
				Service            *string         `json:"service,omitempty"`
				Stack              *string         `json:"stack,omitempty"`
				State              *string         `json:"state,omitempty"`
				Tags               *string         `json:"tags,omitempty"`
			}{Environment: &envID}
		}

		listResp, err := client.Client.GetContainersWithResponse(ctx, params)
		if err != nil {
			return nil, err
		}
		if listResp.JSON200 == nil {
			return nil, apiError("listing containers", listResp.StatusCode(), listResp.JSONDefault)
		}

		all = append(all, listResp.JSON200.Data...)
		if len(listResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}
