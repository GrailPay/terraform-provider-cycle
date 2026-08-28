# List every IP pool in the hub. Counts only — use cycle_pool_ips
# to enumerate individual addresses in a pool.
data "cycle_ip_pools" "all" {}

output "available_ips" {
  value = sum([for pool in data.cycle_ip_pools.all.ip_pools : pool.ips_available])
}
