# Look up an auto-scale group by ID.
data "cycle_autoscale_group" "by_id" {
  id = "651efd54c53f7b6e2c5a9f21"
}

# Or by its slugged identifier.
data "cycle_autoscale_group" "by_identifier" {
  identifier = "workers"
}

output "asg_maximum" {
  value = data.cycle_autoscale_group.by_identifier.scale.up.maximum
}
