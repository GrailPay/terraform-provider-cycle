resource "cycle_pipeline" "deploy" {
  name = "Deploy API"
}

# Trigger keys authenticate programmatic pipeline runs. The secret is
# returned only at create time and is marked sensitive.
resource "cycle_pipeline_trigger_key" "ci" {
  pipeline_id = cycle_pipeline.deploy.id
  name        = "github-actions"
  ips         = ["203.0.113.10"]
}
