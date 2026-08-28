# Look up a single image by ID.
data "cycle_image" "app" {
  id = "651586fca6078e98982dbd90"
}

output "image_state" {
  value = data.cycle_image.app.state
}
