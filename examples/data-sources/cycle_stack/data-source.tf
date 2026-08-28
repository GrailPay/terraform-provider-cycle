# Look up a stack by ID.
data "cycle_stack" "by_id" {
  id = "651efd54c53f7b6e2c5a9f21"
}

# Or by its slugged identifier.
data "cycle_stack" "by_identifier" {
  identifier = "demo"
}

output "stack_state" {
  value = data.cycle_stack.by_id.state
}
