# A globally-scoped variable exposed as an environment variable in
# every container in the environment.
resource "cycle_scoped_variable" "api_token" {
  environment_id = cycle_environment.api.id
  identifier     = "API_TOKEN"
  value          = var.api_token

  access = {
    env_variable = {
      key = "API_TOKEN"
    }
  }
}

# A variable scoped to specific containers, mounted as a file and
# served over the internal API for 5 minutes after runtime start.
resource "cycle_scoped_variable" "tls_cert" {
  environment_id = cycle_environment.api.id
  identifier     = "TLS_CERT"
  value          = base64encode(var.tls_cert_pem)

  scope = {
    container_identifiers = ["api", "worker"]
  }

  access = {
    file = {
      path   = "/etc/ssl/private/tls.pem"
      decode = true
    }
    internal_api = {
      duration = "5m"
    }
  }
}
