package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTLSCertificateResource_basic(t *testing.T) {
	bundle := os.Getenv("CYCLE_ACC_TLS_BUNDLE")
	key := os.Getenv("CYCLE_ACC_TLS_KEY")
	if bundle == "" || key == "" {
		t.Skip("CYCLE_ACC_TLS_BUNDLE and CYCLE_ACC_TLS_KEY must be set to run this acceptance test")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { servicesAccPreCheck(t) },
		ProtoV6ProviderFactories: servicesAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccTLSCertificateResourceConfig(bundle, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_tls_certificate.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_tls_certificate.test", "state"),
					resource.TestCheckResourceAttrSet("cycle_tls_certificate.test", "expires"),
					resource.TestCheckResourceAttr("cycle_tls_certificate.test", "user_supplied", "true"),
					resource.TestCheckResourceAttrSet("data.cycle_tls_certificates.all", "certificates.#"),
				),
			},
			{
				ResourceName:            "cycle_tls_certificate.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"bundle", "private_key"},
			},
		},
	})
}

func testAccTLSCertificateResourceConfig(bundle, key string) string {
	return fmt.Sprintf(`
resource "cycle_tls_certificate" "test" {
  bundle      = %[1]q
  private_key = %[2]q
}

data "cycle_tls_certificates" "all" {}
`, bundle, key)
}
