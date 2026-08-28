package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccDnsRecordResource(t *testing.T) {
	origin := fmt.Sprintf("tf-acc-%s.example.com", acctest.RandString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { dnsImagesPreCheck(t) },
		ProtoV6ProviderFactories: dnsImagesProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDnsRecordConfig(origin, "127.0.0.1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_dns_record.test", "id"),
					resource.TestCheckResourceAttrPair("cycle_dns_record.test", "zone_id", "cycle_dns_zone.test", "id"),
					resource.TestCheckResourceAttr("cycle_dns_record.test", "name", "www"),
					resource.TestCheckResourceAttr("cycle_dns_record.test", "type.a.ip", "127.0.0.1"),
					resource.TestCheckResourceAttrSet("cycle_dns_record.test", "resolved_domain"),
				),
			},
			{
				// Change the A record target to exercise Update.
				Config: testAccDnsRecordConfig(origin, "127.0.0.2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_dns_record.test", "type.a.ip", "127.0.0.2"),
				),
			},
			{
				ResourceName:      "cycle_dns_record.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The record imports via the composite ID "zone_id/record_id".
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["cycle_dns_record.test"]
					if !ok {
						return "", fmt.Errorf("resource cycle_dns_record.test not found in state")
					}
					return rs.Primary.Attributes["zone_id"] + "/" + rs.Primary.ID, nil
				},
				ImportStateVerifyIgnore: []string{"state"},
			},
		},
	})
}

func TestAccDnsRecordResourceMx(t *testing.T) {
	origin := fmt.Sprintf("tf-acc-%s.example.com", acctest.RandString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { dnsImagesPreCheck(t) },
		ProtoV6ProviderFactories: dnsImagesProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "cycle_dns_zone" "test" {
  origin = %[1]q
  hosted = true
}

resource "cycle_dns_record" "mx" {
  zone_id = cycle_dns_zone.test.id
  name    = "@"

  type = {
    mx = {
      priority = 10
      domain   = "mail.%[1]s"
    }
  }
}
`, origin),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_dns_record.mx", "type.mx.priority", "10"),
					resource.TestCheckResourceAttr("cycle_dns_record.mx", "type.mx.domain", "mail."+origin),
				),
			},
		},
	})
}

func testAccDnsRecordConfig(origin, ip string) string {
	return fmt.Sprintf(`
resource "cycle_dns_zone" "test" {
  origin = %[1]q
  hosted = true
}

resource "cycle_dns_record" "test" {
  zone_id = cycle_dns_zone.test.id
  name    = "www"

  type = {
    a = {
      ip = %[2]q
    }
  }
}
`, origin, ip)
}
