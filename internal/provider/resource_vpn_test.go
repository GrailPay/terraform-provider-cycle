package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVPNResource_basic(t *testing.T) {
	envID := os.Getenv("CYCLE_ACC_ENVIRONMENT_ID")
	if envID == "" {
		t.Skip("CYCLE_ACC_ENVIRONMENT_ID is not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { servicesAccPreCheck(t) },
		ProtoV6ProviderFactories: servicesAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccVPNResourceConfig(envID, true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_vpn.test", "environment_id", envID),
					resource.TestCheckResourceAttr("cycle_vpn.test", "enable", "true"),
					resource.TestCheckResourceAttr("cycle_vpn.test", "allow_internet", "false"),
					resource.TestCheckResourceAttrSet("cycle_vpn.test", "id"),
				),
			},
			{
				ResourceName:            "cycle_vpn.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"high_availability"},
			},
			{
				Config: testAccVPNResourceConfig(envID, true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_vpn.test", "allow_internet", "true"),
				),
			},
		},
	})
}

func testAccVPNResourceConfig(environmentID string, enable, allowInternet bool) string {
	return fmt.Sprintf(`
resource "cycle_vpn" "test" {
  environment_id    = %[1]q
  enable            = %[2]t
  high_availability = false
  auto_update       = false
  allow_internet    = %[3]t
  cycle_accounts    = true
  vpn_accounts      = true
}
`, environmentID, enable, allowInternet)
}
