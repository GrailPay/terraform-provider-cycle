# A stateless container. config is required because the create API models it
# as a non-pointer Cycle Config (deploy + network at minimum).
resource "cycle_container" "web" {
  name           = "Web"
  environment_id = cycle_environment.api.id
  image_id       = data.cycle_image.app.id
  stateful       = false

  # Starts the container after create via a job. Create-only; not sent on update.
  start_on_create = true

  config = jsonencode({
    deploy = {
      instances = 1
    }
    network = {
      hostname           = "web"
      public             = "enable"
      egress_via_gateway = false
      ports              = ["80:80"]
    }
  })

  annotations = {
    team = "platform"
  }
}
