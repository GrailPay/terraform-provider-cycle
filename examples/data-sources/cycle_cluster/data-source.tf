# Look up a cluster by its identifier.
data "cycle_cluster" "production" {
  identifier = "production"
}

# Or by its ID.
data "cycle_cluster" "by_id" {
  id = "651efd54c53f7b6e2c5a9f21"
}

resource "cycle_environment" "api" {
  name    = "API"
  cluster = data.cycle_cluster.production.identifier
}
