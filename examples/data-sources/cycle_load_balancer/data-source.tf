data "cycle_load_balancer" "api" {
  environment_id = cycle_environment.api.id
}

output "lb_container_id" {
  value = data.cycle_load_balancer.api.container_id
}
