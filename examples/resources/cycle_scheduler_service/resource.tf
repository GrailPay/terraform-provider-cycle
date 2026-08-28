# The scheduler is an environment singleton used by function containers.
# Destroy only drops Terraform state; the Cycle scheduler service remains.
resource "cycle_scheduler_service" "api" {
  environment_id = cycle_environment.api.id
  auto_update    = true

  config = jsonencode({
    public = false
  })
}
