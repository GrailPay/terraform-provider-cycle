# List every stack in the hub.
data "cycle_stacks" "all" {}

output "stack_names" {
  value = [for s in data.cycle_stacks.all.stacks : s.name]
}

output "live_stack_ids" {
  value = [
    for s in data.cycle_stacks.all.stacks : s.id
    if s.state == "live"
  ]
}
