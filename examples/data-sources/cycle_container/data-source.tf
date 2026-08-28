# Look up a container by ID.
data "cycle_container" "web" {
  id = "651efd54c53f7b6e2c5a9f21"
}

output "container_state" {
  value = data.cycle_container.web.state
}
