# List every server in the hub.
data "cycle_servers" "all" {}

# Or only those in a specific cluster.
data "cycle_servers" "production" {
  cluster = "production"
}

output "live_server_hostnames" {
  value = [
    for srv in data.cycle_servers.production.servers : srv.hostname
    if srv.state == "live"
  ]
}
