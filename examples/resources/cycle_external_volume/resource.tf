# Attach an existing SAN iSCSI LUN to a server. `source` and `attachment`
# are JSON objects matching the Cycle API unions.
resource "cycle_external_volume" "data" {
  name        = "app-data"
  identifier  = "app-data"
  cluster     = "production"
  location_id = var.location_id
  server_ids  = [cycle_server.worker.id]
  description = "Persistent data volume for the API"

  source = jsonencode({
    type = "san-iscsi"
    details = {
      integration_ids = [var.iscsi_integration_id]
      lun             = 0
    }
  })

  attachment = jsonencode({
    type    = "block"
    mode    = "single-node-writer"
    details = {}
  })

  options = {
    create = {
      size = "100GB"
    }
  }
}
