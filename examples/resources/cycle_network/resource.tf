# An SDN network that can attach environments in the same cluster.
resource "cycle_network" "shared" {
  name       = "shared"
  identifier = "shared"
  cluster    = "production"

  environment_ids = [
    cycle_environment.api.id,
    cycle_environment.worker.id,
  ]

  # Optional ACL: role ID → view / modify / manage.
  acl = jsonencode({
    roles = {
      "651efd54c53f7b6e2c5a9f00" = {
        view   = true
        modify = false
        manage = false
      }
    }
  })
}
