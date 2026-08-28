# Hub webhooks are a singleton on the current hub.
resource "cycle_hub_webhooks" "current" {
  server_deployed = "https://hooks.example.com/cycle/server-deployed"
  server_deleted  = "https://hooks.example.com/cycle/server-deleted"
}
