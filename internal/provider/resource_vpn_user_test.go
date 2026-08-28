package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccVPNUserResource_basic(t *testing.T) {
	envID := os.Getenv("CYCLE_ACC_ENVIRONMENT_ID")
	if envID == "" {
		t.Skip("CYCLE_ACC_ENVIRONMENT_ID is not set")
	}

	username := acctest.RandomWithPrefix("tf-acc-vpn")
	updatedUsername := username + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { servicesAccPreCheck(t) },
		ProtoV6ProviderFactories: servicesAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccVPNUserResourceConfig(envID, username, "s3cret-vpn-pass"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_vpn_user.test", "environment_id", envID),
					resource.TestCheckResourceAttr("cycle_vpn_user.test", "username", username),
					resource.TestCheckResourceAttrSet("cycle_vpn_user.test", "id"),
				),
			},
			{
				ResourceName:            "cycle_vpn_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["cycle_vpn_user.test"]
					if !ok {
						return "", fmt.Errorf("resource cycle_vpn_user.test not found in state")
					}
					return rs.Primary.Attributes["environment_id"] + "/" + rs.Primary.Attributes["username"], nil
				},
			},
			{
				Config: testAccVPNUserResourceConfig(envID, updatedUsername, "s3cret-vpn-pass"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_vpn_user.test", "username", updatedUsername),
					resource.TestCheckResourceAttrSet("cycle_vpn_user.test", "id"),
				),
			},
		},
	})
}

func testAccVPNUserResourceConfig(environmentID, username, password string) string {
	return fmt.Sprintf(`
resource "cycle_vpn_user" "test" {
  environment_id = %[1]q
  username       = %[2]q
  password       = %[3]q
}
`, environmentID, username, password)
}
