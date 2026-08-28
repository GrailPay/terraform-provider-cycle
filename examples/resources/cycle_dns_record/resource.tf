resource "cycle_dns_zone" "example" {
  origin = "example.com"
  hosted = true
}

# A simple A record: www.example.com -> 203.0.113.10
resource "cycle_dns_record" "www" {
  zone_id = cycle_dns_zone.example.id
  name    = "www"

  type = {
    a = {
      ip = "203.0.113.10"
    }
  }
}

# An MX record on the zone root.
resource "cycle_dns_record" "mail" {
  zone_id = cycle_dns_zone.example.id
  name    = "@"

  type = {
    mx = {
      priority = 10
      domain   = "mail.example.com"
    }
  }
}

# A LINKED record pointing at a container, with automatic TLS.
resource "cycle_dns_record" "app" {
  zone_id = cycle_dns_zone.example.id
  name    = "app"

  type = {
    linked = {
      features = {
        tls = {
          enable = true
        }
      }

      container_id = "651586fca6078e98982dbd90"
    }
  }
}
