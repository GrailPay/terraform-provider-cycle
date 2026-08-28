# Deployment versions and tags for an environment. Versions appear when
# containers set deployment.version. Tags (prod, dev, ...) point at a version
# and are used by DNS LINKED records.
data "cycle_environment_deployments" "api" {
  environment_id = cycle_environment.api.id
}

output "deployment_versions" {
  value = [
    for v in data.cycle_environment_deployments.api.versions : {
      version    = v.version
      containers = v.containers
      tags       = v.tags
    }
  ]
}

# Look up the version a tag currently points at.
output "prod_version" {
  value = try(data.cycle_environment_deployments.api.tags["prod"], null)
}
