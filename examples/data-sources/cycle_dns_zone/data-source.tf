# Look up a DNS zone by its origin (domain name)...
data "cycle_dns_zone" "by_origin" {
  origin = "example.com"
}

# ...or by its ID.
data "cycle_dns_zone" "by_id" {
  id = "651586fca6078e98982dbd90"
}

output "zone_hosted" {
  value = data.cycle_dns_zone.by_origin.hosted
}
