package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/grailpay/terraform-provider-cycle/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// dnsImagesProviderFactories is the provider factory map shared by the DNS
// and image acceptance tests in this package.
var dnsImagesProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"cycle": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// dnsImagesPreCheck verifies the environment variables required to run the
// DNS and image acceptance tests against a real hub.
func dnsImagesPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("CYCLE_API_KEY") == "" {
		t.Fatal("CYCLE_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("CYCLE_HUB_ID") == "" {
		t.Fatal("CYCLE_HUB_ID must be set for acceptance tests")
	}
}

func TestAccDnsZoneResource(t *testing.T) {
	origin := fmt.Sprintf("tf-acc-%s.example.com", acctest.RandString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { dnsImagesPreCheck(t) },
		ProtoV6ProviderFactories: dnsImagesProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDnsZoneConfig(origin, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_dns_zone.test", "id"),
					resource.TestCheckResourceAttr("cycle_dns_zone.test", "origin", origin),
					resource.TestCheckResourceAttr("cycle_dns_zone.test", "hosted", "true"),
					resource.TestCheckResourceAttrSet("cycle_dns_zone.test", "state"),
				),
			},
			{
				// Flip hosted to exercise Update.
				Config: testAccDnsZoneConfig(origin, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_dns_zone.test", "hosted", "false"),
				),
			},
			{
				ResourceName:      "cycle_dns_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
				// state can transition between refreshes (e.g. pending -> live).
				ImportStateVerifyIgnore: []string{"state"},
			},
		},
	})
}

func TestAccDnsZoneDataSource(t *testing.T) {
	origin := fmt.Sprintf("tf-acc-%s.example.com", acctest.RandString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { dnsImagesPreCheck(t) },
		ProtoV6ProviderFactories: dnsImagesProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDnsZoneConfig(origin, true) + `
data "cycle_dns_zone" "by_origin" {
  origin = cycle_dns_zone.test.origin
}

data "cycle_dns_zone" "by_id" {
  id = cycle_dns_zone.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.cycle_dns_zone.by_origin", "id", "cycle_dns_zone.test", "id"),
					resource.TestCheckResourceAttrPair("data.cycle_dns_zone.by_id", "origin", "cycle_dns_zone.test", "origin"),
					resource.TestCheckResourceAttr("data.cycle_dns_zone.by_origin", "hosted", "true"),
				),
			},
		},
	})
}

func testAccDnsZoneConfig(origin string, hosted bool) string {
	return fmt.Sprintf(`
resource "cycle_dns_zone" "test" {
  origin = %[1]q
  hosted = %[2]t
}
`, origin, hosted)
}
