data "cycle_scoped_variables" "api" {
  environment_id = cycle_environment.api.id
}

output "scoped_variable_identifiers" {
  value = [for v in data.cycle_scoped_variables.api.variables : v.identifier]
}
