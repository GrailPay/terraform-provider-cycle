package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewExternalVolumeDataSource)
}

var (
	_ datasource.DataSource              = (*externalVolumeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*externalVolumeDataSource)(nil)
)

// NewExternalVolumeDataSource returns the cycle_external_volume data source.
func NewExternalVolumeDataSource() datasource.DataSource {
	return &externalVolumeDataSource{}
}

type externalVolumeDataSource struct {
	client *CycleClient
}

type externalVolumeDataSourceModel struct {
	ID          types.String         `tfsdk:"id"`
	Name        types.String         `tfsdk:"name"`
	Cluster     types.String         `tfsdk:"cluster"`
	LocationID  types.String         `tfsdk:"location_id"`
	ServerIDs   types.List           `tfsdk:"server_ids"`
	Identifier  types.String         `tfsdk:"identifier"`
	Description types.String         `tfsdk:"description"`
	Source      jsontypes.Normalized `tfsdk:"source"`
	Attachment  jsontypes.Normalized `tfsdk:"attachment"`
	Size        types.String         `tfsdk:"size"`
	State       types.String         `tfsdk:"state"`
	HubID       types.String         `tfsdk:"hub_id"`
}

func (d *externalVolumeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_volume"
}

func (d *externalVolumeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Cycle external volume by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique ID of the external volume.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name of the external volume.",
			},
			"cluster": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The identifier of the cluster this volume is associated with.",
			},
			"location_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The provider location ID of the volume.",
			},
			"server_ids": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Server IDs that this volume can be mounted on.",
			},
			"identifier": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The human-readable slugged identifier of the volume.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The custom description of the volume.",
			},
			"source": schema.StringAttribute{
				Computed:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "JSON object describing the volume source.",
			},
			"attachment": schema.StringAttribute{
				Computed:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "JSON object describing the volume attachment, if any.",
			},
			"size": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current size of the volume as reported by the API.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current lifecycle state of the volume.",
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this volume belongs to.",
			},
		},
	}
}

func (d *externalVolumeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *externalVolumeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config externalVolumeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Client.GetExternalVolumeWithResponse(ctx, config.ID.ValueString(), &cycle.GetExternalVolumeParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading external volume", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"External Volume Not Found",
			fmt.Sprintf("No external volume with ID %q exists in this hub.", config.ID.ValueString()),
		)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading external volume", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	vol := getResp.JSON200.Data
	config.ID = types.StringValue(vol.Id)
	config.Name = types.StringValue(vol.Name)
	config.Cluster = types.StringValue(vol.Cluster)
	config.LocationID = types.StringValue(vol.LocationId)
	config.HubID = types.StringValue(vol.HubId)
	config.State = types.StringValue(string(vol.State.Current))
	config.Description = types.StringValue(vol.About.Description)
	if vol.Identifier != nil {
		config.Identifier = types.StringValue(*vol.Identifier)
	} else {
		config.Identifier = types.StringNull()
	}
	if vol.Size != nil {
		config.Size = types.StringValue(*vol.Size)
	} else {
		config.Size = types.StringNull()
	}

	ids, diags := types.ListValueFrom(ctx, types.StringType, vol.ServerIds)
	resp.Diagnostics.Append(diags...)
	config.ServerIDs = ids

	sourceJSON, err := json.Marshal(vol.Source)
	if err != nil {
		resp.Diagnostics.AddError("Error encoding external volume source", err.Error())
		return
	}
	config.Source = jsontypes.NewNormalizedValue(string(sourceJSON))

	if vol.Attachment != nil {
		attachmentJSON, err := json.Marshal(vol.Attachment)
		if err != nil {
			resp.Diagnostics.AddError("Error encoding external volume attachment", err.Error())
			return
		}
		config.Attachment = jsontypes.NewNormalizedValue(string(attachmentJSON))
	} else {
		config.Attachment = jsontypes.NewNormalizedNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
