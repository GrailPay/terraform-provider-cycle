package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLoadBalancerResource_basic(t *testing.T) {
	envID := os.Getenv("CYCLE_ACC_ENVIRONMENT_ID")
	if envID == "" {
		t.Skip("CYCLE_ACC_ENVIRONMENT_ID is not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { servicesAccPreCheck(t) },
		ProtoV6ProviderFactories: servicesAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccLoadBalancerResourceConfig(envID, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_load_balancer.test", "environment_id", envID),
					resource.TestCheckResourceAttr("cycle_load_balancer.test", "high_availability", "false"),
					resource.TestCheckResourceAttr("cycle_load_balancer.test", "auto_update", "false"),
					resource.TestCheckResourceAttrSet("cycle_load_balancer.test", "id"),
					resource.TestCheckResourceAttrSet("data.cycle_load_balancer.test", "id"),
				),
			},
			{
				ResourceName:            "cycle_load_balancer.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config"},
			},
			{
				Config: testAccLoadBalancerResourceConfig(envID, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_load_balancer.test", "auto_update", "true"),
				),
			},
		},
	})
}

func testAccLoadBalancerResourceConfig(environmentID string, autoUpdate bool) string {
	return fmt.Sprintf(`
resource "cycle_load_balancer" "test" {
  environment_id    = %[1]q
  high_availability = false
  auto_update       = %[2]t
}

data "cycle_load_balancer" "test" {
  environment_id = cycle_load_balancer.test.environment_id
}
`, environmentID, autoUpdate)
}
