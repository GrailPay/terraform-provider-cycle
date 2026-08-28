data "cycle_image_sources" "all" {}

output "direct_source_ids" {
  value = [
    for source in data.cycle_image_sources.all.sources : source.id
    if source.type == "direct"
  ]
}
