resource "cycle_cluster" "production" {
  identifier = "production"
}

resource "cycle_cluster" "staging" {
  identifier = "staging"

  # Non-essential clusters are excluded by default from certain
  # metrics and summaries.
  non_essential = true
}
