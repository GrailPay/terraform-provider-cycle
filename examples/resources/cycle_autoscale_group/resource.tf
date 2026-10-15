resource "cycle_autoscale_group" "workers" {
  name       = "Workers"
  identifier = "workers"
  cluster    = "production"

  infrastructure = [
    {
      provider       = "equinix-metal"
      model_id       = var.model_id
      integration_id = var.provider_integration_id
      priority       = 1
      locations = [
        {
          id                 = var.location_id
          availability_zones = []
        }
      ]
    }
  ]

  scale = {
    up = {
      maximum = 3
    }
    down = {
      inactivity_period = "1h"
      method            = "fifo"
      min_ttl           = "24h"
    }
  }

  # Applied to every server this group creates. encrypt_storage is not
  # available on the group API.
  provision = {
    overcommit          = true
    overcommit_multiple = 15
    pool                = true
    services            = true
    tags                = []
    base_volume_gb      = 100
    sftp                = false
  }
}
