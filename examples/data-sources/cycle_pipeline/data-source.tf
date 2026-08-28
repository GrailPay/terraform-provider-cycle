# Look up a pipeline by ID.
data "cycle_pipeline" "deploy" {
  id = "651efd54c53f7b6e2c5a9f21"
}

output "pipeline_state" {
  value = data.cycle_pipeline.deploy.state
}
