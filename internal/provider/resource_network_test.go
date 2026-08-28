package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNetworkResource_basic(t *testing.T) {
	cluster := os.Getenv("CYCLE_ACC_CLUSTER")
	if cluster == "" {
		t.Skip("CYCLE_ACC_CLUSTER is not set")
	}

	name := acctest.RandomWithPrefix("tf-acc-net")
	updated := name + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkResourceConfig(name, name, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_network.test", "id"),
					resource.TestCheckResourceAttr("cycle_network.test", "name", name),
					resource.TestCheckResourceAttr("cycle_network.test", "identifier", name),
					resource.TestCheckResourceAttr("cycle_network.test", "cluster", cluster),
					resource.TestCheckResourceAttr("cycle_network.test", "environment_ids.#", "0"),
					resource.TestCheckResourceAttrSet("cycle_network.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_network.test", "state"),
					resource.TestCheckResourceAttrSet("data.cycle_networks.all", "networks.#"),
					resource.TestCheckResourceAttrPair("data.cycle_network.by_id", "id", "cycle_network.test", "id"),
					resource.TestCheckResourceAttrPair("data.cycle_network.by_id", "name", "cycle_network.test", "name"),
				),
			},
			{
				ResourceName:      "cycle_network.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccNetworkResourceConfig(updated, name, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_network.test", "name", updated),
					resource.TestCheckResourceAttr("cycle_network.test", "identifier", name),
				),
			},
		},
	})
}

func testAccNetworkResourceConfig(name, identifier, cluster string) string {
	return fmt.Sprintf(`
resource "cycle_network" "test" {
  name       = %[1]q
  identifier = %[2]q
  cluster    = %[3]q
}

data "cycle_network" "by_id" {
  id = cycle_network.test.id
}

data "cycle_networks" "all" {}
`, name, identifier, cluster)
}
