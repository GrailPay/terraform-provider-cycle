# Look up an integration by ID.
data "cycle_integration" "aws" {
  id = "651efd54c53f7b6e2c5a9f21"
}

output "integration_state" {
  value = data.cycle_integration.aws.state
}
