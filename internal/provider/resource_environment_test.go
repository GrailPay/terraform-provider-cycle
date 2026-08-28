package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnvironmentResource_basic(t *testing.T) {
	clusterIdentifier := acctest.RandomWithPrefix("tf-acc-cluster")
	name := acctest.RandomWithPrefix("tf-acc-env")
	updatedName := name + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { clusterEnvAccPreCheck(t) },
		ProtoV6ProviderFactories: clusterEnvAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentResourceConfig(clusterIdentifier, name, "created by acceptance test"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_environment.test", "name", name),
					resource.TestCheckResourceAttr("cycle_environment.test", "cluster", clusterIdentifier),
					resource.TestCheckResourceAttr("cycle_environment.test", "description", "created by acceptance test"),
					resource.TestCheckResourceAttr("cycle_environment.test", "legacy_networking", "false"),
					resource.TestCheckResourceAttrSet("cycle_environment.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_environment.test", "identifier"),
					resource.TestCheckResourceAttrSet("cycle_environment.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_environment.test", "state"),
				),
			},
			{
				ResourceName:      "cycle_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccEnvironmentResourceConfig(clusterIdentifier, updatedName, "updated by acceptance test"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_environment.test", "name", updatedName),
					resource.TestCheckResourceAttr("cycle_environment.test", "description", "updated by acceptance test"),
				),
			},
		},
	})
}

func testAccEnvironmentResourceConfig(clusterIdentifier, name, description string) string {
	return fmt.Sprintf(`
resource "cycle_cluster" "test" {
  identifier = %[1]q
}

resource "cycle_environment" "test" {
  name        = %[2]q
  cluster     = cycle_cluster.test.identifier
  description = %[3]q
}
`, clusterIdentifier, name, description)
}
