# List every image in the hub.
data "cycle_images" "all" {}

# List only images built from a specific image source.
data "cycle_images" "from_source" {
  source_id = cycle_image_source.docker_hub.id
}

output "image_ids" {
  value = [for image in data.cycle_images.from_source.images : image.id]
}
