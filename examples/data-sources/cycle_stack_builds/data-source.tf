# All builds for a stack.
data "cycle_stack_builds" "all" {
  stack_id = cycle_stack.demo.id
}

# Only live builds — useful when pointing a pipeline at an existing build
# instead of creating a new cycle_stack_build.
data "cycle_stack_builds" "live" {
  stack_id = cycle_stack.demo.id
  state    = "live"
}

output "latest_live_build_id" {
  value = try(data.cycle_stack_builds.live.builds[0].id, null)
}
