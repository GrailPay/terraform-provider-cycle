# Look up an environment by name.
data "cycle_environment" "api" {
  name = "API"
}

# Or by its slugged identifier.
data "cycle_environment" "by_identifier" {
  identifier = "api"
}

# Or by its ID.
data "cycle_environment" "by_id" {
  id = "651efd54c53f7b6e2c5a9f21"
}

resource "cycle_scoped_variable" "api_token" {
  environment_id = data.cycle_environment.api.id
  identifier     = "API_TOKEN"
  value          = var.api_token
}
