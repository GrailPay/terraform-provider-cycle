package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDiscoveryServiceResource_basic(t *testing.T) {
	envID := os.Getenv("CYCLE_ACC_ENVIRONMENT_ID")
	if envID == "" {
		t.Skip("CYCLE_ACC_ENVIRONMENT_ID is not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { servicesAccPreCheck(t) },
		ProtoV6ProviderFactories: servicesAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentServicesConfig(envID, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_discovery_service.test", "environment_id", envID),
					resource.TestCheckResourceAttr("cycle_discovery_service.test", "id", envID),
					resource.TestCheckResourceAttr("cycle_discovery_service.test", "auto_update", "false"),
					resource.TestCheckResourceAttrSet("cycle_discovery_service.test", "enable"),
					resource.TestCheckResourceAttr("cycle_gateway_service.test", "environment_id", envID),
					resource.TestCheckResourceAttr("cycle_gateway_service.test", "id", envID),
					resource.TestCheckResourceAttr("cycle_scheduler_service.test", "environment_id", envID),
					resource.TestCheckResourceAttr("cycle_scheduler_service.test", "id", envID),
				),
			},
			{
				ResourceName:      "cycle_discovery_service.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "cycle_gateway_service.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "cycle_scheduler_service.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccEnvironmentServicesConfig(envID, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_discovery_service.test", "auto_update", "true"),
				),
			},
		},
	})
}

func testAccEnvironmentServicesConfig(environmentID string, autoUpdate bool) string {
	return fmt.Sprintf(`
resource "cycle_discovery_service" "test" {
  environment_id    = %[1]q
  auto_update       = %[2]t
  high_availability = false
}

resource "cycle_gateway_service" "test" {
  environment_id = %[1]q
  auto_update    = false
}

resource "cycle_scheduler_service" "test" {
  environment_id = %[1]q
  auto_update    = false
}
`, environmentID, autoUpdate)
}
