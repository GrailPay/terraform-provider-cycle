# List every role on the current hub.
data "cycle_hub_roles" "all" {}

# Find the built-in owner role's ID.
output "owner_role_id" {
  value = one([for role in data.cycle_hub_roles.all.roles : role.id if role.root])
}
