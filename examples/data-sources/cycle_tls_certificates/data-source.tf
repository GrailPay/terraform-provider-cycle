# List user-supplied TLS certificates. Private material is never returned.
data "cycle_tls_certificates" "all" {}

output "certificate_ids" {
  value = [for c in data.cycle_tls_certificates.all.certificates : c.id]
}

output "live_domains" {
  value = flatten([
    for c in data.cycle_tls_certificates.all.certificates : c.domains
    if c.state == "live"
  ])
}
