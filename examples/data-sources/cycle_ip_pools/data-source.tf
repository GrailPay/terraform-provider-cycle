# List every IP pool in the hub. The API reports available/total counts
# only — it does not enumerate individual addresses.
data "cycle_ip_pools" "all" {}

output "available_ips" {
  value = sum([for pool in data.cycle_ip_pools.all.ip_pools : pool.ips_available])
}
