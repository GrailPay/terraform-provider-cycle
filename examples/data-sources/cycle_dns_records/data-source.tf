data "cycle_dns_records" "public" {
  zone_id = cycle_dns_zone.public.id
}

output "linked_prod_records" {
  value = [
    for record in data.cycle_dns_records.public.records : record.resolved_domain
    if record.record_type == "linked" && try(record.linked.tag, null) == "prod"
  ]
}
