# Look up a single IP pool by ID.
data "cycle_ip_pool" "public" {
  id = "651efd54c53f7b6e2c5a9f21"
}

output "pool_cidr" {
  value = data.cycle_ip_pool.public.cidr
}
