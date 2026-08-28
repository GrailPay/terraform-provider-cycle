package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDnsZonesDataSource_list(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { clusterEnvAccPreCheck(t) },
		ProtoV6ProviderFactories: clusterEnvAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `data "cycle_dns_zones" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.cycle_dns_zones.all", "zones.#"),
				),
			},
		},
	})
}

func TestAccDnsRecordsDataSource_list(t *testing.T) {
	zoneID := os.Getenv("CYCLE_ACC_ZONE_ID")
	if zoneID == "" {
		t.Skip("CYCLE_ACC_ZONE_ID is not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { clusterEnvAccPreCheck(t) },
		ProtoV6ProviderFactories: clusterEnvAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "cycle_dns_records" "test" {
  zone_id = %q
}
`, zoneID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cycle_dns_records.test", "zone_id", zoneID),
					resource.TestCheckResourceAttrSet("data.cycle_dns_records.test", "records.#"),
				),
			},
		},
	})
}
