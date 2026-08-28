# A zone fully hosted by Cycle — Cycle serves all record types.
resource "cycle_dns_zone" "hosted" {
  origin = "example.com"
  hosted = true
}

# A linked zone — DNS stays with your existing provider and
# Cycle only manages LINKED records.
resource "cycle_dns_zone" "linked" {
  origin = "linked.example.com"
  hosted = false
}
