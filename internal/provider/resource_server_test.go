package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccServerResource_basic(t *testing.T) {
	infraAccRequireEnv(t,
		"CYCLE_ACC_CLUSTER",
		"CYCLE_ACC_INTEGRATION_ID",
		"CYCLE_ACC_LOCATION_ID",
		"CYCLE_ACC_MODEL_ID",
	)

	nickname := acctest.RandomWithPrefix("tf-acc-server")
	updated := nickname + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerResourceConfig(nickname),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_server.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_server.test", "hostname"),
					resource.TestCheckResourceAttrSet("cycle_server.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_server.test", "state"),
					resource.TestCheckResourceAttr("cycle_server.test", "cluster", os.Getenv("CYCLE_ACC_CLUSTER")),
					resource.TestCheckResourceAttr("cycle_server.test", "nickname", nickname),
				),
			},
			{
				ResourceName:            "cycle_server.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force_delete", "zone", "attached_storage_size", "encrypt_storage", "reservation_id"},
			},
			{
				Config: testAccServerResourceConfig(updated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_server.test", "nickname", updated),
				),
			},
		},
	})
}

func TestAccServerDataSources(t *testing.T) {
	infraAccRequireEnv(t,
		"CYCLE_ACC_CLUSTER",
		"CYCLE_ACC_INTEGRATION_ID",
		"CYCLE_ACC_LOCATION_ID",
		"CYCLE_ACC_MODEL_ID",
	)

	nickname := acctest.RandomWithPrefix("tf-acc-server-ds")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerResourceConfig(nickname) + `
data "cycle_server" "by_id" {
  id = cycle_server.test.id
}

data "cycle_servers" "in_cluster" {
  cluster = cycle_server.test.cluster
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.cycle_server.by_id", "hostname", "cycle_server.test", "hostname"),
					resource.TestCheckResourceAttrPair("data.cycle_servers.in_cluster", "cluster", "cycle_server.test", "cluster"),
				),
			},
		},
	})
}

func TestAccServersDataSource_list(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `data "cycle_servers" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.cycle_servers.all", "servers.#"),
				),
			},
		},
	})
}

func testAccServerResourceConfig(nickname string) string {
	return fmt.Sprintf(`
resource "cycle_server" "test" {
  cluster        = %[1]q
  integration_id = %[2]q
  location_id    = %[3]q
  model_id       = %[4]q
  nickname       = %[5]q
  force_delete   = true
}
`, os.Getenv("CYCLE_ACC_CLUSTER"), os.Getenv("CYCLE_ACC_INTEGRATION_ID"), os.Getenv("CYCLE_ACC_LOCATION_ID"), os.Getenv("CYCLE_ACC_MODEL_ID"), nickname)
}
