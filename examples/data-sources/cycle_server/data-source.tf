# Look up a server by ID.
data "cycle_server" "worker" {
  id = "651efd54c53f7b6e2c5a9f21"
}

output "worker_hostname" {
  value = data.cycle_server.worker.hostname
}
