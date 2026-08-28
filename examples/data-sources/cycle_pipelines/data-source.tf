# List every pipeline in the hub.
data "cycle_pipelines" "all" {}

output "pipeline_names" {
  value = [for p in data.cycle_pipelines.all.pipelines : p.name]
}

output "live_pipeline_ids" {
  value = [
    for p in data.cycle_pipelines.all.pipelines : p.id
    if p.state == "live"
  ]
}
