package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewDnsRecordResource)
}

var (
	_ resource.Resource                   = (*dnsRecordResource)(nil)
	_ resource.ResourceWithConfigure      = (*dnsRecordResource)(nil)
	_ resource.ResourceWithImportState    = (*dnsRecordResource)(nil)
	_ resource.ResourceWithValidateConfig = (*dnsRecordResource)(nil)
)

// NewDnsRecordResource returns the cycle_dns_record resource.
func NewDnsRecordResource() resource.Resource {
	return &dnsRecordResource{}
}

type dnsRecordResource struct {
	client *CycleClient
}

type dnsRecordResourceModel struct {
	ID             types.String        `tfsdk:"id"`
	ZoneID         types.String        `tfsdk:"zone_id"`
	Name           types.String        `tfsdk:"name"`
	ResolvedDomain types.String        `tfsdk:"resolved_domain"`
	State          types.String        `tfsdk:"state"`
	Type           *dnsRecordTypeModel `tfsdk:"type"`
}

type dnsRecordTypeModel struct {
	A      *dnsRecordAModel      `tfsdk:"a"`
	AAAA   *dnsRecordAModel      `tfsdk:"aaaa"`
	CNAME  *dnsRecordDomainModel `tfsdk:"cname"`
	NS     *dnsRecordDomainModel `tfsdk:"ns"`
	ALIAS  *dnsRecordDomainModel `tfsdk:"alias"`
	TXT    *dnsRecordTxtModel    `tfsdk:"txt"`
	MX     *dnsRecordMxModel     `tfsdk:"mx"`
	SRV    *dnsRecordSrvModel    `tfsdk:"srv"`
	CAA    *dnsRecordCaaModel    `tfsdk:"caa"`
	Linked *dnsRecordLinkedModel `tfsdk:"linked"`
}

type dnsRecordAModel struct {
	IP types.String `tfsdk:"ip"`
}

type dnsRecordDomainModel struct {
	Domain types.String `tfsdk:"domain"`
}

type dnsRecordTxtModel struct {
	Value types.String `tfsdk:"value"`
}

type dnsRecordMxModel struct {
	Priority types.Int64  `tfsdk:"priority"`
	Domain   types.String `tfsdk:"domain"`
}

type dnsRecordSrvModel struct {
	Weight   types.Int64  `tfsdk:"weight"`
	Priority types.Int64  `tfsdk:"priority"`
	Port     types.Int64  `tfsdk:"port"`
	Domain   types.String `tfsdk:"domain"`
}

type dnsRecordCaaModel struct {
	Tag   types.String `tfsdk:"tag"`
	Value types.String `tfsdk:"value"`
}

type dnsRecordLinkedModel struct {
	Features       *dnsRecordLinkedFeaturesModel   `tfsdk:"features"`
	ContainerID    types.String                    `tfsdk:"container_id"`
	Deployment     *dnsRecordLinkedDeploymentModel `tfsdk:"deployment"`
	VirtualMachine *dnsRecordLinkedVMModel         `tfsdk:"virtual_machine"`
}

type dnsRecordLinkedFeaturesModel struct {
	TLS      *dnsRecordFeatureEnableModel  `tfsdk:"tls"`
	GeoDNS   *dnsRecordFeatureEnableModel  `tfsdk:"geodns"`
	Wildcard *dnsRecordLinkedWildcardModel `tfsdk:"wildcard"`
}

type dnsRecordFeatureEnableModel struct {
	Enable types.Bool `tfsdk:"enable"`
}

type dnsRecordLinkedWildcardModel struct {
	ResolveSubDomains types.Bool `tfsdk:"resolve_sub_domains"`
}

type dnsRecordLinkedDeploymentModel struct {
	EnvironmentID types.String                         `tfsdk:"environment_id"`
	Match         *dnsRecordLinkedDeploymentMatchModel `tfsdk:"match"`
}

type dnsRecordLinkedDeploymentMatchModel struct {
	Container types.String `tfsdk:"container"`
	Tag       types.String `tfsdk:"tag"`
}

type dnsRecordLinkedVMModel struct {
	ID  types.String `tfsdk:"id"`
	DMZ types.Bool   `tfsdk:"dmz"`
}

func (r *dnsRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *dnsRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	enableAttr := func(desc string) schema.Attribute {
		return schema.BoolAttribute{
			Required:            true,
			MarkdownDescription: desc,
		}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DNS record within a Cycle DNS zone. Exactly one record type must be set " +
			"under the `type` attribute, mirroring the Cycle API's record type union " +
			"(A, AAAA, CNAME, NS, MX, TXT, ALIAS, SRV, CAA, or LINKED).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the DNS record.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone_id": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The ID of the DNS zone this record belongs to. Changing this forces a new " +
					"record to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The name of the record, where `@` signifies the root of the zone's origin. " +
					"The Cycle API does not support renaming records, so changing this forces a new record to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resolved_domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The fully-qualified domain name resolved from the record name and the zone origin.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the DNS record (e.g. `pending`, `live`).",
			},
			"type": schema.SingleNestedAttribute{
				Required: true,
				MarkdownDescription: "The record type. Exactly one of the nested record types must be set, " +
					"mirroring the Cycle API's oneOf record union.",
				Attributes: map[string]schema.Attribute{
					"a": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "A DNS A record, mapping the name to an IPv4 address.",
						Attributes: map[string]schema.Attribute{
							"ip": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The IPv4 address the A record maps to.",
							},
						},
					},
					"aaaa": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "A DNS AAAA record, mapping the name to an IPv6 address.",
						Attributes: map[string]schema.Attribute{
							"ip": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The IPv6 address the AAAA record maps to.",
							},
						},
					},
					"cname": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "A DNS CNAME record, aliasing the name to another domain.",
						Attributes: map[string]schema.Attribute{
							"domain": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The domain the CNAME record resolves to.",
							},
						},
					},
					"ns": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "A DNS NS record, delegating the name to a nameserver.",
						Attributes: map[string]schema.Attribute{
							"domain": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The domain of the nameserver for this record.",
							},
						},
					},
					"alias": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "A DNS ALIAS record, returning the target domain's records at this name.",
						Attributes: map[string]schema.Attribute{
							"domain": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The domain returned from the DNS server when this alias record is requested.",
							},
						},
					},
					"txt": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "A DNS TXT record.",
						Attributes: map[string]schema.Attribute{
							"value": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The value of the TXT record.",
							},
						},
					},
					"mx": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "A DNS MX record for mail routing.",
						Attributes: map[string]schema.Attribute{
							"priority": schema.Int64Attribute{
								Required:            true,
								MarkdownDescription: "The priority of this MX record; lower values are preferred.",
							},
							"domain": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The mail server domain this MX record points to.",
							},
						},
					},
					"srv": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "A DNS SRV record for service discovery.",
						Attributes: map[string]schema.Attribute{
							"weight": schema.Int64Attribute{
								Required:            true,
								MarkdownDescription: "The weight of the record — breaks ties for priority.",
							},
							"priority": schema.Int64Attribute{
								Required:            true,
								MarkdownDescription: "The priority of the record; lower values are preferred.",
							},
							"port": schema.Int64Attribute{
								Required:            true,
								MarkdownDescription: "The port number of the service.",
							},
							"domain": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The domain providing the service.",
							},
						},
					},
					"caa": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "A DNS CAA record, restricting which certificate authorities may issue certificates for the domain.",
						Attributes: map[string]schema.Attribute{
							"tag": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The property identifier of the record, e.g. `issue` or `issuewild`.",
							},
							"value": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The value associated with the tag, e.g. a certificate authority domain.",
							},
						},
					},
					"linked": schema.SingleNestedAttribute{
						Optional: true,
						MarkdownDescription: "A Cycle LINKED record, pointing the name at a container, deployment, or " +
							"virtual machine. Cycle manages the underlying IP mapping automatically. Exactly one of " +
							"`container_id`, `deployment`, or `virtual_machine` must be set.",
						Attributes: map[string]schema.Attribute{
							"features": schema.SingleNestedAttribute{
								Required:            true,
								MarkdownDescription: "Platform features applied to this LINKED record.",
								Attributes: map[string]schema.Attribute{
									"tls": schema.SingleNestedAttribute{
										Required:            true,
										MarkdownDescription: "TLS settings for the record.",
										Attributes: map[string]schema.Attribute{
											"enable": enableAttr("When `true`, Cycle automatically provisions and maintains a TLS certificate for this record."),
										},
									},
									"geodns": schema.SingleNestedAttribute{
										Optional:            true,
										Computed:            true,
										MarkdownDescription: "GeoDNS settings for the record. Defaults to disabled.",
										Default: objectdefault.StaticValue(types.ObjectValueMust(
											map[string]attr.Type{"enable": types.BoolType},
											map[string]attr.Value{"enable": types.BoolValue(false)},
										)),
										Attributes: map[string]schema.Attribute{
											"enable": enableAttr("When `true`, Cycle attempts to route inbound requests to the geographically closest load balancer."),
										},
									},
									"wildcard": schema.SingleNestedAttribute{
										Optional:            true,
										MarkdownDescription: "Wildcard settings for the record.",
										Attributes: map[string]schema.Attribute{
											"resolve_sub_domains": schema.BoolAttribute{
												Required: true,
												MarkdownDescription: "When `true`, subdomains resolve for wildcard records. " +
													"When `false`, only the primary domain resolves.",
											},
										},
									},
								},
							},
							"container_id": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "The ID of the container this LINKED record points to.",
							},
							"deployment": schema.SingleNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Points this LINKED record at a tagged deployment within an environment.",
								Attributes: map[string]schema.Attribute{
									"environment_id": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The ID of the environment with the deployment tag mapping to reference.",
									},
									"match": schema.SingleNestedAttribute{
										Required:            true,
										MarkdownDescription: "Which container and tagged deployment this record targets.",
										Attributes: map[string]schema.Attribute{
											"container": schema.StringAttribute{
												Required:            true,
												MarkdownDescription: "The identifier of the container in the environment this record points to.",
											},
											"tag": schema.StringAttribute{
												Optional: true,
												MarkdownDescription: "The deployment tag this record points to. Tags are set on the " +
													"environment root and map to a deployment version.",
											},
										},
									},
								},
							},
							"virtual_machine": schema.SingleNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Points this LINKED record at a virtual machine.",
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The ID of the virtual machine.",
									},
									"dmz": schema.BoolAttribute{
										Optional: true,
										Computed: true,
										Default:  booldefault.StaticBool(false),
										MarkdownDescription: "When `true`, traffic to this domain skips the load balancer and goes " +
											"directly to the virtual machine via the gateway service. Defaults to `false`.",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *dnsRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

// ValidateConfig enforces the API's oneOf semantics: exactly one record type
// under `type`, and for LINKED records exactly one destination.
func (r *dnsRecordResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var typeObj types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("type"), &typeObj)...)
	if resp.Diagnostics.HasError() || typeObj.IsNull() || typeObj.IsUnknown() {
		return
	}

	variantNames := []string{"a", "aaaa", "cname", "ns", "alias", "txt", "mx", "srv", "caa", "linked"}
	if !exactlyOneAttrSet(typeObj, variantNames) {
		resp.Diagnostics.AddAttributeError(
			path.Root("type"),
			"Invalid DNS Record Type",
			fmt.Sprintf("Exactly one record type must be set under `type`: one of %s.", strings.Join(variantNames, ", ")),
		)
		return
	}

	linkedVal, ok := typeObj.Attributes()["linked"]
	if !ok || linkedVal.IsNull() || linkedVal.IsUnknown() {
		return
	}
	linkedObj, ok := linkedVal.(types.Object)
	if !ok {
		return
	}
	destNames := []string{"container_id", "deployment", "virtual_machine"}
	if !exactlyOneAttrSet(linkedObj, destNames) {
		resp.Diagnostics.AddAttributeError(
			path.Root("type").AtName("linked"),
			"Invalid LINKED Record Destination",
			fmt.Sprintf("Exactly one destination must be set on a LINKED record: one of %s.", strings.Join(destNames, ", ")),
		)
	}
}

// exactlyOneAttrSet reports whether exactly one of the named attributes on obj
// is non-null. Unknown values are treated as set, since their final value
// cannot be determined until apply.
func exactlyOneAttrSet(obj types.Object, names []string) bool {
	count := 0
	for _, name := range names {
		v, ok := obj.Attributes()[name]
		if ok && !v.IsNull() {
			count++
		}
	}
	return count == 1
}

func (r *dnsRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordType, err := expandDnsRecordType(plan.Type)
	if err != nil {
		resp.Diagnostics.AddError("Error building DNS record type", err.Error())
		return
	}

	apiResp, err := r.client.Client.CreateDNSZoneRecordWithResponse(ctx, plan.ZoneID.ValueString(),
		&cycle.CreateDNSZoneRecordParams{},
		cycle.CreateDNSZoneRecordJSONRequestBody{
			Name: plan.Name.ValueString(),
			Type: recordType,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNS record", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating DNS record", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(flattenDnsRecord(apiResp.JSON201.Data, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.GetDnsRecordWithResponse(ctx, state.ZoneID.ValueString(), state.ID.ValueString(), &cycle.GetDnsRecordParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS record", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading DNS record", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	record := apiResp.JSON200.Data
	if record.State.Current == cycle.DnsRecordStateCurrentDeleted || record.State.Current == cycle.DnsRecordStateCurrentDeleting {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(flattenDnsRecord(record, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordType, err := expandDnsRecordType(plan.Type)
	if err != nil {
		resp.Diagnostics.AddError("Error building DNS record type", err.Error())
		return
	}

	apiResp, err := r.client.Client.UpdateDNSZoneRecordWithResponse(ctx, plan.ZoneID.ValueString(), plan.ID.ValueString(),
		&cycle.UpdateDNSZoneRecordParams{},
		cycle.UpdateDNSZoneRecordJSONRequestBody{Type: recordType},
	)
	if err != nil {
		resp.Diagnostics.AddError("Error updating DNS record", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating DNS record", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(flattenDnsRecord(apiResp.JSON200.Data, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.DeleteDNSZoneRecordWithResponse(ctx, state.ZoneID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting DNS record", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return
	}
	if apiResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting DNS record", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	// Record deletion may spawn an async job (e.g. certificate cleanup).
	if data := apiResp.JSON202.Data; data != nil && data.Job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, data.Job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for DNS record deletion", err.Error())
		}
	}
}

// ImportState imports a record via the composite ID "zone_id/record_id".
func (r *dnsRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected an import ID in the form \"zone_id/record_id\", got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// expandDnsRecordType converts the Terraform model into the API's record type union.
func expandDnsRecordType(m *dnsRecordTypeModel) (cycle.DnsRecordTypes, error) {
	var t cycle.DnsRecordTypes
	if m == nil {
		return t, fmt.Errorf("record type is required")
	}

	switch {
	case m.A != nil:
		t.A = &cycle.DnsRecordA{Ip: m.A.IP.ValueString()}
	case m.AAAA != nil:
		t.Aaaa = &cycle.DnsRecordAaaa{Ip: m.AAAA.IP.ValueString()}
	case m.CNAME != nil:
		t.Cname = &cycle.DnsRecordCname{Domain: m.CNAME.Domain.ValueString()}
	case m.NS != nil:
		t.Ns = &cycle.DnsRecordNs{Domain: m.NS.Domain.ValueString()}
	case m.ALIAS != nil:
		t.Alias = &cycle.DnsRecordAlias{Domain: m.ALIAS.Domain.ValueString()}
	case m.TXT != nil:
		t.Txt = &cycle.DnsRecordTxt{Value: m.TXT.Value.ValueString()}
	case m.MX != nil:
		t.Mx = &cycle.DnsRecordMx{
			Priority: int(m.MX.Priority.ValueInt64()),
			Domain:   m.MX.Domain.ValueString(),
		}
	case m.SRV != nil:
		t.Srv = &cycle.DnsRecordSrv{
			Weight:   int(m.SRV.Weight.ValueInt64()),
			Priority: int(m.SRV.Priority.ValueInt64()),
			Port:     int(m.SRV.Port.ValueInt64()),
			Domain:   m.SRV.Domain.ValueString(),
		}
	case m.CAA != nil:
		t.Caa = &cycle.DnsRecordCaa{
			Tag:   m.CAA.Tag.ValueString(),
			Value: m.CAA.Value.ValueString(),
		}
	case m.Linked != nil:
		linked, err := expandDnsRecordLinked(m.Linked)
		if err != nil {
			return t, err
		}
		t.Linked = linked
	default:
		return t, fmt.Errorf("exactly one record type must be set under `type`")
	}

	return t, nil
}

func expandDnsRecordLinked(m *dnsRecordLinkedModel) (*cycle.DnsRecordLinked, error) {
	linked := &cycle.DnsRecordLinked{}

	if m.Features == nil || m.Features.TLS == nil {
		return nil, fmt.Errorf("linked records require features.tls")
	}
	linked.Features.Tls.Enable = m.Features.TLS.Enable.ValueBool()
	if m.Features.GeoDNS != nil {
		linked.Features.Geodns.Enable = m.Features.GeoDNS.Enable.ValueBool()
	}
	if m.Features.Wildcard != nil {
		linked.Features.Wildcard = &struct {
			ResolveSubDomains bool `json:"resolve_sub_domains"`
		}{ResolveSubDomains: m.Features.Wildcard.ResolveSubDomains.ValueBool()}
	}

	switch {
	case !m.ContainerID.IsNull():
		containerID := m.ContainerID.ValueString()
		if err := linked.FromDnsRecordLinked0(cycle.DnsRecordLinked0{ContainerId: &containerID}); err != nil {
			return nil, err
		}
	case m.Deployment != nil:
		if m.Deployment.Match == nil {
			return nil, fmt.Errorf("linked deployment records require a match block")
		}
		dep := cycle.DnsRecordLinkedDeployment{
			Deployment: &struct {
				EnvironmentId string `json:"environment_id"`
				Match         struct {
					Container string  `json:"container"`
					Tag       *string `json:"tag,omitempty"`
				} `json:"match"`
			}{},
		}
		dep.Deployment.EnvironmentId = m.Deployment.EnvironmentID.ValueString()
		dep.Deployment.Match.Container = m.Deployment.Match.Container.ValueString()
		if !m.Deployment.Match.Tag.IsNull() {
			tag := m.Deployment.Match.Tag.ValueString()
			dep.Deployment.Match.Tag = &tag
		}
		if err := linked.FromDnsRecordLinkedDeployment(dep); err != nil {
			return nil, err
		}
	case m.VirtualMachine != nil:
		var vm cycle.DnsRecordLinkedVirtualMachine
		vm.VirtualMachine.Id = m.VirtualMachine.ID.ValueString()
		vm.VirtualMachine.Dmz = m.VirtualMachine.DMZ.ValueBool()
		if err := linked.FromDnsRecordLinkedVirtualMachine(vm); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("exactly one of container_id, deployment, or virtual_machine must be set on a linked record")
	}

	return linked, nil
}

// flattenDnsRecord maps an API record onto the Terraform model.
func flattenDnsRecord(record cycle.DnsRecord, m *dnsRecordResourceModel) (diags diag.Diagnostics) {
	m.ID = types.StringValue(record.Id)
	m.ZoneID = types.StringValue(record.ZoneId)
	m.Name = types.StringValue(record.Name)
	m.ResolvedDomain = types.StringValue(record.ResolvedDomain)
	m.State = types.StringValue(string(record.State.Current))

	flattened, err := flattenDnsRecordType(record.Type)
	if err != nil {
		diags.AddError("Error reading DNS record type", err.Error())
		return diags
	}
	m.Type = flattened
	return diags
}

func flattenDnsRecordType(t cycle.DnsRecordTypes) (*dnsRecordTypeModel, error) {
	m := &dnsRecordTypeModel{}

	switch {
	case t.A != nil:
		m.A = &dnsRecordAModel{IP: types.StringValue(t.A.Ip)}
	case t.Aaaa != nil:
		m.AAAA = &dnsRecordAModel{IP: types.StringValue(t.Aaaa.Ip)}
	case t.Cname != nil:
		m.CNAME = &dnsRecordDomainModel{Domain: types.StringValue(t.Cname.Domain)}
	case t.Ns != nil:
		m.NS = &dnsRecordDomainModel{Domain: types.StringValue(t.Ns.Domain)}
	case t.Alias != nil:
		m.ALIAS = &dnsRecordDomainModel{Domain: types.StringValue(t.Alias.Domain)}
	case t.Txt != nil:
		m.TXT = &dnsRecordTxtModel{Value: types.StringValue(t.Txt.Value)}
	case t.Mx != nil:
		m.MX = &dnsRecordMxModel{
			Priority: types.Int64Value(int64(t.Mx.Priority)),
			Domain:   types.StringValue(t.Mx.Domain),
		}
	case t.Srv != nil:
		m.SRV = &dnsRecordSrvModel{
			Weight:   types.Int64Value(int64(t.Srv.Weight)),
			Priority: types.Int64Value(int64(t.Srv.Priority)),
			Port:     types.Int64Value(int64(t.Srv.Port)),
			Domain:   types.StringValue(t.Srv.Domain),
		}
	case t.Caa != nil:
		m.CAA = &dnsRecordCaaModel{
			Tag:   types.StringValue(t.Caa.Tag),
			Value: types.StringValue(t.Caa.Value),
		}
	case t.Linked != nil:
		linked, err := flattenDnsRecordLinked(*t.Linked)
		if err != nil {
			return nil, err
		}
		m.Linked = linked
	default:
		return nil, fmt.Errorf("the Cycle API returned a DNS record with no recognized type")
	}

	return m, nil
}

func flattenDnsRecordLinked(linked cycle.DnsRecordLinked) (*dnsRecordLinkedModel, error) {
	m := &dnsRecordLinkedModel{
		Features: &dnsRecordLinkedFeaturesModel{
			TLS:    &dnsRecordFeatureEnableModel{Enable: types.BoolValue(linked.Features.Tls.Enable)},
			GeoDNS: &dnsRecordFeatureEnableModel{Enable: types.BoolValue(linked.Features.Geodns.Enable)},
		},
	}
	if linked.Features.Wildcard != nil {
		m.Features.Wildcard = &dnsRecordLinkedWildcardModel{
			ResolveSubDomains: types.BoolValue(linked.Features.Wildcard.ResolveSubDomains),
		}
	}

	// The linked destination union has no discriminator, so probe variants in
	// order of specificity: deployment, then virtual machine, then container.
	if dep, err := linked.AsDnsRecordLinkedDeployment(); err == nil && dep.Deployment != nil {
		m.Deployment = &dnsRecordLinkedDeploymentModel{
			EnvironmentID: types.StringValue(dep.Deployment.EnvironmentId),
			Match: &dnsRecordLinkedDeploymentMatchModel{
				Container: types.StringValue(dep.Deployment.Match.Container),
				Tag:       types.StringPointerValue(dep.Deployment.Match.Tag),
			},
		}
		return m, nil
	}

	if vm, err := linked.AsDnsRecordLinkedVirtualMachine(); err == nil && vm.VirtualMachine.Id != "" {
		m.VirtualMachine = &dnsRecordLinkedVMModel{
			ID:  types.StringValue(vm.VirtualMachine.Id),
			DMZ: types.BoolValue(vm.VirtualMachine.Dmz),
		}
		return m, nil
	}

	if c, err := linked.AsDnsRecordLinked0(); err == nil && c.ContainerId != nil {
		m.ContainerID = types.StringValue(*c.ContainerId)
		return m, nil
	}

	return nil, fmt.Errorf("the Cycle API returned a LINKED record with no recognized destination")
}
