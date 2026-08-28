package provider

import (
	"context"
	"fmt"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterDataSource(NewDnsRecordsDataSource)
}

var (
	_ datasource.DataSource              = (*dnsRecordsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dnsRecordsDataSource)(nil)
)

// NewDnsRecordsDataSource returns the cycle_dns_records data source.
func NewDnsRecordsDataSource() datasource.DataSource {
	return &dnsRecordsDataSource{}
}

type dnsRecordsDataSource struct {
	client *CycleClient
}

type dnsRecordsDataSourceModel struct {
	ZoneID  types.String             `tfsdk:"zone_id"`
	Records []dnsRecordDataItemModel `tfsdk:"records"`
}

type dnsRecordDataItemModel struct {
	ID             types.String              `tfsdk:"id"`
	ZoneID         types.String              `tfsdk:"zone_id"`
	Name           types.String              `tfsdk:"name"`
	ResolvedDomain types.String              `tfsdk:"resolved_domain"`
	State          types.String              `tfsdk:"state"`
	RecordType     types.String              `tfsdk:"record_type"`
	Value          types.String              `tfsdk:"value"`
	Linked         *dnsRecordDataLinkedModel `tfsdk:"linked"`
}

type dnsRecordDataLinkedModel struct {
	ContainerID      types.String `tfsdk:"container_id"`
	EnvironmentID    types.String `tfsdk:"environment_id"`
	Container        types.String `tfsdk:"container"`
	Tag              types.String `tfsdk:"tag"`
	VirtualMachineID types.String `tfsdk:"virtual_machine_id"`
}

func (d *dnsRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_records"
}

func (d *dnsRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists DNS records in a Cycle DNS zone. Each record includes a `record_type` (`a`, `aaaa`, `cname`, `ns`, `alias`, `txt`, `mx`, `srv`, `caa`, or `linked`) and, for LINKED records, the destination used by deployment tags.",
		Attributes: map[string]schema.Attribute{
			"zone_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the DNS zone whose records should be listed.",
			},
			"records": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The records in the zone.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique ID of the DNS record.",
						},
						"zone_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the DNS zone this record belongs to.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The record name, where `@` is the zone origin.",
						},
						"resolved_domain": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The fully-qualified domain name resolved from the record name and zone origin.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The current state of the DNS record.",
						},
						"record_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The record type: `a`, `aaaa`, `cname`, `ns`, `alias`, `txt`, `mx`, `srv`, `caa`, or `linked`.",
						},
						"value": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The primary value for non-LINKED records (IP, domain, TXT value, or CAA value). Null for LINKED records.",
						},
						"linked": schema.SingleNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Present when `record_type` is `linked`. Describes the container, deployment tag, or virtual machine the record points at.",
							Attributes: map[string]schema.Attribute{
								"container_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The container ID, when the record is linked directly to a container.",
								},
								"environment_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The environment ID, when the record is linked to a deployment.",
								},
								"container": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The container identifier matched by a deployment LINKED record.",
								},
								"tag": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The deployment tag matched by a deployment LINKED record (for example `prod`).",
								},
								"virtual_machine_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The virtual machine ID, when the record is linked to a VM.",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *dnsRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *dnsRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config dnsRecordsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	records, err := fetchAllDNSZoneRecords(ctx, d.client, config.ZoneID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing DNS records", err.Error())
		return
	}

	config.Records = make([]dnsRecordDataItemModel, 0, len(records))
	for _, record := range records {
		item, diags := dnsRecordDataItemFromAPI(record)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.Records = append(config.Records, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func fetchAllDNSZoneRecords(ctx context.Context, client *CycleClient, zoneID string) ([]cycle.DnsRecord, error) {
	const pageSize = 100

	var all []cycle.DnsRecord
	for pageNumber := float32(1); ; pageNumber++ {
		number := pageNumber
		size := float32(pageSize)
		apiResp, err := client.Client.GetDNSZoneRecordsWithResponse(ctx, zoneID, &cycle.GetDNSZoneRecordsParams{
			Page: &cycle.PageParam{Number: &number, Size: &size},
		})
		if err != nil {
			return nil, err
		}
		if apiResp.JSON200 == nil {
			return nil, apiError("listing DNS records", apiResp.StatusCode(), apiResp.JSONDefault)
		}

		all = append(all, apiResp.JSON200.Data...)
		if len(apiResp.JSON200.Data) < pageSize {
			return all, nil
		}
	}
}

func dnsRecordDataItemFromAPI(record cycle.DnsRecord) (dnsRecordDataItemModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	item := dnsRecordDataItemModel{
		ID:             types.StringValue(record.Id),
		ZoneID:         types.StringValue(record.ZoneId),
		Name:           types.StringValue(record.Name),
		ResolvedDomain: types.StringValue(record.ResolvedDomain),
		State:          types.StringValue(string(record.State.Current)),
		Value:          types.StringNull(),
	}

	t := record.Type
	switch {
	case t.A != nil:
		item.RecordType = types.StringValue("a")
		item.Value = types.StringValue(t.A.Ip)
	case t.Aaaa != nil:
		item.RecordType = types.StringValue("aaaa")
		item.Value = types.StringValue(t.Aaaa.Ip)
	case t.Cname != nil:
		item.RecordType = types.StringValue("cname")
		item.Value = types.StringValue(t.Cname.Domain)
	case t.Ns != nil:
		item.RecordType = types.StringValue("ns")
		item.Value = types.StringValue(t.Ns.Domain)
	case t.Alias != nil:
		item.RecordType = types.StringValue("alias")
		item.Value = types.StringValue(t.Alias.Domain)
	case t.Txt != nil:
		item.RecordType = types.StringValue("txt")
		item.Value = types.StringValue(t.Txt.Value)
	case t.Mx != nil:
		item.RecordType = types.StringValue("mx")
		item.Value = types.StringValue(t.Mx.Domain)
	case t.Srv != nil:
		item.RecordType = types.StringValue("srv")
		item.Value = types.StringValue(t.Srv.Domain)
	case t.Caa != nil:
		item.RecordType = types.StringValue("caa")
		item.Value = types.StringValue(t.Caa.Value)
	case t.Linked != nil:
		item.RecordType = types.StringValue("linked")
		linked, err := flattenDnsRecordLinked(*t.Linked)
		if err != nil {
			diags.AddError("Error reading DNS record type", fmt.Sprintf("record %s: %s", record.Id, err.Error()))
			return item, diags
		}
		item.Linked = &dnsRecordDataLinkedModel{
			ContainerID:      types.StringNull(),
			EnvironmentID:    types.StringNull(),
			Container:        types.StringNull(),
			Tag:              types.StringNull(),
			VirtualMachineID: types.StringNull(),
		}
		if !linked.ContainerID.IsNull() {
			item.Linked.ContainerID = linked.ContainerID
		}
		if linked.Deployment != nil {
			item.Linked.EnvironmentID = linked.Deployment.EnvironmentID
			if linked.Deployment.Match != nil {
				item.Linked.Container = linked.Deployment.Match.Container
				item.Linked.Tag = linked.Deployment.Match.Tag
			}
		}
		if linked.VirtualMachine != nil {
			item.Linked.VirtualMachineID = linked.VirtualMachine.ID
		}
	default:
		diags.AddError("Error reading DNS record type", fmt.Sprintf("record %s: the Cycle API returned no recognized type", record.Id))
	}

	return item, diags
}
