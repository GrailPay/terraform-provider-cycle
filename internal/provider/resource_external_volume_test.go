package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccExternalVolumeResource_basic(t *testing.T) {
	infraAccRequireEnv(t,
		"CYCLE_ACC_CLUSTER",
		"CYCLE_ACC_LOCATION_ID",
		"CYCLE_ACC_SERVER_ID",
		"CYCLE_ACC_VOLUME_SOURCE",
		"CYCLE_ACC_VOLUME_ATTACHMENT",
	)

	name := acctest.RandomWithPrefix("tf-acc-volume")
	updated := name + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccExternalVolumeResourceConfig(name, "created by acceptance test"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_external_volume.test", "id"),
					resource.TestCheckResourceAttr("cycle_external_volume.test", "name", name),
					resource.TestCheckResourceAttr("cycle_external_volume.test", "description", "created by acceptance test"),
					resource.TestCheckResourceAttrSet("cycle_external_volume.test", "state"),
					resource.TestCheckResourceAttrSet("cycle_external_volume.test", "hub_id"),
				),
			},
			{
				ResourceName:            "cycle_external_volume.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_source_device"},
			},
			{
				Config: testAccExternalVolumeResourceConfig(updated, "updated by acceptance test"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_external_volume.test", "name", updated),
					resource.TestCheckResourceAttr("cycle_external_volume.test", "description", "updated by acceptance test"),
				),
			},
		},
	})
}

func TestAccExternalVolumeDataSource(t *testing.T) {
	infraAccRequireEnv(t,
		"CYCLE_ACC_CLUSTER",
		"CYCLE_ACC_LOCATION_ID",
		"CYCLE_ACC_SERVER_ID",
		"CYCLE_ACC_VOLUME_SOURCE",
		"CYCLE_ACC_VOLUME_ATTACHMENT",
	)

	name := acctest.RandomWithPrefix("tf-acc-volume-ds")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccExternalVolumeResourceConfig(name, "data source lookup") + `
data "cycle_external_volume" "by_id" {
  id = cycle_external_volume.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.cycle_external_volume.by_id", "name", "cycle_external_volume.test", "name"),
					resource.TestCheckResourceAttrPair("data.cycle_external_volume.by_id", "cluster", "cycle_external_volume.test", "cluster"),
				),
			},
		},
	})
}

func testAccExternalVolumeResourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "cycle_external_volume" "test" {
  name        = %[1]q
  cluster     = %[2]q
  location_id = %[3]q
  server_ids  = [%[4]q]
  description = %[5]q
  source      = %[6]q
  attachment  = %[7]q
  options     = {}
}
`, name, os.Getenv("CYCLE_ACC_CLUSTER"), os.Getenv("CYCLE_ACC_LOCATION_ID"), os.Getenv("CYCLE_ACC_SERVER_ID"), description, os.Getenv("CYCLE_ACC_VOLUME_SOURCE"), os.Getenv("CYCLE_ACC_VOLUME_ATTACHMENT"))
}
