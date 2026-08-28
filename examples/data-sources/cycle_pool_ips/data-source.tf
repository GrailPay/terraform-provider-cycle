data "cycle_pool_ips" "cluster" {
  pool_id = data.cycle_ip_pool.cluster.id
}

output "unassigned_addresses" {
  value = [
    for ip in data.cycle_pool_ips.cluster.ips : ip.address
    if ip.container_id == null && ip.virtual_machine_id == null
  ]
}
