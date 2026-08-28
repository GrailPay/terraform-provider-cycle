resource "cycle_vpn_user" "deploy" {
  environment_id = cycle_environment.api.id
  username       = "deploy"
  password       = var.vpn_deploy_password
}

variable "vpn_deploy_password" {
  type        = string
  sensitive   = true
  description = "Password for the deploy VPN user."
}
