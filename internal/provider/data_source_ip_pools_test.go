package provider_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccIPPoolsDataSource_list(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `data "cycle_ip_pools" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.cycle_ip_pools.all", "ip_pools.#"),
				),
			},
		},
	})
}

func TestAccIPPoolDataSource_byID(t *testing.T) {
	infraAccRequireEnv(t, "CYCLE_ACC_IP_POOL_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `
data "cycle_ip_pool" "one" {
  id = "` + os.Getenv("CYCLE_ACC_IP_POOL_ID") + `"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cycle_ip_pool.one", "id", os.Getenv("CYCLE_ACC_IP_POOL_ID")),
					resource.TestCheckResourceAttrSet("data.cycle_ip_pool.one", "cidr"),
					resource.TestCheckResourceAttrSet("data.cycle_ip_pool.one", "ips_total"),
				),
			},
		},
	})
}
