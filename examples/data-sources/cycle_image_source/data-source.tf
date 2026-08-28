# Look up an image source by its human-readable identifier...
data "cycle_image_source" "by_identifier" {
  identifier = "nginx"
}

# ...or by its ID.
data "cycle_image_source" "by_id" {
  id = "651586fca6078e98982dbd90"
}

output "image_source_type" {
  value = data.cycle_image_source.by_identifier.type
}
