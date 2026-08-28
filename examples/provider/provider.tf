terraform {
  required_providers {
    cycle = {
      source  = "tomschlick/cycle"
      version = "~> 0.1"
    }
  }
}

provider "cycle" {
  # Both attributes may be omitted and provided via the
  # CYCLE_API_KEY and CYCLE_HUB_ID environment variables instead.
  api_key = var.cycle_api_key
  hub_id  = var.cycle_hub_id
}

variable "cycle_api_key" {
  type        = string
  sensitive   = true
  description = "Cycle API key."
}

variable "cycle_hub_id" {
  type        = string
  description = "ID of the Cycle hub to manage."
}
