package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAutoscaleGroupResource_basic(t *testing.T) {
	infraAccRequireEnv(t,
		"CYCLE_ACC_CLUSTER",
		"CYCLE_ACC_INTEGRATION_ID",
		"CYCLE_ACC_LOCATION_ID",
		"CYCLE_ACC_MODEL_ID",
		"CYCLE_ACC_PROVIDER",
	)

	name := acctest.RandomWithPrefix("tf-acc-asg")
	identifier := acctest.RandomWithPrefix("tf-acc-asg")
	updated := name + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccAutoscaleGroupResourceConfig(name, identifier, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_autoscale_group.test", "id"),
					resource.TestCheckResourceAttr("cycle_autoscale_group.test", "name", name),
					resource.TestCheckResourceAttr("cycle_autoscale_group.test", "identifier", identifier),
					resource.TestCheckResourceAttr("cycle_autoscale_group.test", "cluster", os.Getenv("CYCLE_ACC_CLUSTER")),
					resource.TestCheckResourceAttr("cycle_autoscale_group.test", "scale.up.maximum", "1"),
					resource.TestCheckResourceAttrSet("cycle_autoscale_group.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_autoscale_group.test", "state"),
				),
			},
			{
				ResourceName:      "cycle_autoscale_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAutoscaleGroupResourceConfig(updated, identifier, 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_autoscale_group.test", "name", updated),
					resource.TestCheckResourceAttr("cycle_autoscale_group.test", "scale.up.maximum", "2"),
				),
			},
		},
	})
}

func TestAccAutoscaleGroupDataSource(t *testing.T) {
	infraAccRequireEnv(t,
		"CYCLE_ACC_CLUSTER",
		"CYCLE_ACC_INTEGRATION_ID",
		"CYCLE_ACC_LOCATION_ID",
		"CYCLE_ACC_MODEL_ID",
		"CYCLE_ACC_PROVIDER",
	)

	name := acctest.RandomWithPrefix("tf-acc-asg-ds")
	identifier := acctest.RandomWithPrefix("tf-acc-asg-ds")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccAutoscaleGroupResourceConfig(name, identifier, 1) + `
data "cycle_autoscale_group" "by_id" {
  id = cycle_autoscale_group.test.id
}

data "cycle_autoscale_group" "by_identifier" {
  identifier = cycle_autoscale_group.test.identifier
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.cycle_autoscale_group.by_id", "name", "cycle_autoscale_group.test", "name"),
					resource.TestCheckResourceAttrPair("data.cycle_autoscale_group.by_identifier", "id", "cycle_autoscale_group.test", "id"),
				),
			},
		},
	})
}

func testAccAutoscaleGroupResourceConfig(name, identifier string, maximum int) string {
	return fmt.Sprintf(`
resource "cycle_autoscale_group" "test" {
  name       = %[1]q
  identifier = %[2]q
  cluster    = %[3]q

  infrastructure = [
    {
      provider       = %[4]q
      model_id       = %[5]q
      integration_id = %[6]q
      priority       = 1
      locations = [
        {
          id                 = %[7]q
          availability_zones = []
        }
      ]
    }
  ]

  scale = {
    up = {
      maximum = %[8]d
    }
    down = {
      inactivity_period = "1h"
      method            = "fifo"
      min_ttl           = "1h"
    }
  }
}
`, name, identifier, os.Getenv("CYCLE_ACC_CLUSTER"), os.Getenv("CYCLE_ACC_PROVIDER"), os.Getenv("CYCLE_ACC_MODEL_ID"), os.Getenv("CYCLE_ACC_INTEGRATION_ID"), os.Getenv("CYCLE_ACC_LOCATION_ID"), maximum)
}
