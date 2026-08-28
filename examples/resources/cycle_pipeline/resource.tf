# A pipeline with no stages. Stages can be added later as normalized JSON.
resource "cycle_pipeline" "empty" {
  name = "Bootstrap"
}

# A disabled pipeline with a single empty stage. `dynamic` is one-way: once
# enabled it cannot be turned back off.
resource "cycle_pipeline" "deploy" {
  name       = "Deploy API"
  disable    = false
  dynamic    = true
  identifier = "deploy-api"

  stages = jsonencode([
    {
      identifier = "build"
      steps      = []
    }
  ])
}
