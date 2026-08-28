# Upload a user-supplied certificate. Cycle has no hard delete — destroy
# issues a deprecate job so the platform can fall back to another cert.
resource "cycle_tls_certificate" "custom" {
  bundle      = file("${path.module}/cert.pem")
  private_key = file("${path.module}/key.pem")
}
