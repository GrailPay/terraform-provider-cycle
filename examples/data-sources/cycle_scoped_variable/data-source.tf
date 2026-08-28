# Look up a scoped variable by identifier in an environment.
data "cycle_scoped_variable" "api_token" {
  environment_id = cycle_environment.api.id
  identifier     = "API_TOKEN"
}

# Or by its ID.
data "cycle_scoped_variable" "by_id" {
  environment_id = cycle_environment.api.id
  id             = "651efd54c53f7b6e2c5a9f21"
}

output "api_token_key" {
  value = try(data.cycle_scoped_variable.api_token.access.env_variable.key, null)
}
