package provider

import (
	"context"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewDnsZoneResource)
}

var (
	_ resource.Resource                = (*dnsZoneResource)(nil)
	_ resource.ResourceWithConfigure   = (*dnsZoneResource)(nil)
	_ resource.ResourceWithImportState = (*dnsZoneResource)(nil)
)

// NewDnsZoneResource returns the cycle_dns_zone resource.
func NewDnsZoneResource() resource.Resource {
	return &dnsZoneResource{}
}

type dnsZoneResource struct {
	client *CycleClient
}

type dnsZoneResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Origin types.String `tfsdk:"origin"`
	Hosted types.Bool   `tfsdk:"hosted"`
	State  types.String `tfsdk:"state"`
}

func (r *dnsZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone"
}

func (r *dnsZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle DNS zone. A zone represents a domain (origin) whose DNS records " +
			"are either fully hosted by Cycle (`hosted = true`) or managed externally with only LINKED records " +
			"handled by Cycle (`hosted = false`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the DNS zone.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"origin": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The origin (domain name) of the DNS zone, e.g. `example.com`. " +
					"Changing this forces a new zone to be created — zones cannot be renamed.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"hosted": schema.BoolAttribute{
				Required: true,
				MarkdownDescription: "Whether this zone is hosted by Cycle. When `true`, Cycle acts as the " +
					"authoritative nameserver for the origin and serves all record types. When `false`, the zone " +
					"is linked: DNS stays with your existing provider and Cycle only manages LINKED records.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the DNS zone (e.g. `new`, `pending`, `verifying`, `live`, `disabled`).",
			},
		},
	}
}

func (r *dnsZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *dnsZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.CreateDNSZoneWithResponse(ctx, cycle.CreateDNSZoneJSONRequestBody{
		Origin: plan.Origin.ValueString(),
		Hosted: plan.Hosted.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNS zone", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating DNS zone", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	flattenDnsZone(apiResp.JSON201.Data, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.GetDNSZoneWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading DNS zone", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	zone := apiResp.JSON200.Data
	if zone.State.Current == cycle.DnsZoneStateCurrentDeleted || zone.State.Current == cycle.DnsZoneStateCurrentDeleting {
		resp.State.RemoveResource(ctx)
		return
	}

	flattenDnsZone(zone, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnsZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hosted := plan.Hosted.ValueBool()
	apiResp, err := r.client.Client.UpdateDNSZoneWithResponse(ctx, plan.ID.ValueString(),
		&cycle.UpdateDNSZoneParams{},
		cycle.UpdateDNSZoneJSONRequestBody{Hosted: &hosted},
	)
	if err != nil {
		resp.Diagnostics.AddError("Error updating DNS zone", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating DNS zone", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	flattenDnsZone(apiResp.JSON200.Data, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.DeleteDNSZoneWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting DNS zone", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return
	}
	if apiResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting DNS zone", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	// Zone deletion is asynchronous; wait for the job so dependent resources
	// (e.g. re-creating the same origin) don't race the deletion.
	if job := apiResp.JSON202.Data.Job; job != nil {
		if _, err := waitForJob(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for DNS zone deletion", err.Error())
		}
	}
}

func (r *dnsZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenDnsZone(zone cycle.DnsZone, m *dnsZoneResourceModel) {
	m.ID = types.StringValue(zone.Id)
	m.Origin = types.StringValue(zone.Origin)
	m.Hosted = types.BoolValue(zone.Hosted)
	m.State = types.StringValue(string(zone.State.Current))
}
