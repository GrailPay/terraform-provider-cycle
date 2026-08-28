# Look up a stack build by ID.
data "cycle_stack_build" "by_id" {
  stack_id = cycle_stack.demo.id
  id       = "651efd54c53f7b6e2c5a9f21"
}

# Or by the version set on the build.
data "cycle_stack_build" "by_version" {
  stack_id = cycle_stack.demo.id
  version  = "1.2.3"
}

output "live_build_state" {
  value = data.cycle_stack_build.by_version.state
}
