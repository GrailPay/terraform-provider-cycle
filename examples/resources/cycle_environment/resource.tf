resource "cycle_cluster" "production" {
  identifier = "production"
}

resource "cycle_environment" "api" {
  name        = "API"
  cluster     = cycle_cluster.production.identifier
  description = "Production API environment"

  # Optional; auto-generated from the name if omitted.
  identifier = "api"

  # Create-only: changing this forces a new environment.
  legacy_networking = false
}
