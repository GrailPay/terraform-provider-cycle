# List every member of the current hub with their account email and role.
data "cycle_hub_members" "all" {}

output "member_emails" {
  value = [for member in data.cycle_hub_members.all.members : member.email]
}
