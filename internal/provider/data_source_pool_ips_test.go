package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPoolIPsDataSource_list(t *testing.T) {
	infraAccRequireEnv(t, "CYCLE_ACC_IP_POOL_ID")

	poolID := os.Getenv("CYCLE_ACC_IP_POOL_ID")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "cycle_pool_ips" "test" {
  pool_id = %q
}
`, poolID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cycle_pool_ips.test", "pool_id", poolID),
					resource.TestCheckResourceAttrSet("data.cycle_pool_ips.test", "ips.#"),
				),
			},
		},
	})
}
