package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVPNDataSource_basic(t *testing.T) {
	envID := os.Getenv("CYCLE_ACC_ENVIRONMENT_ID")
	if envID == "" {
		t.Skip("CYCLE_ACC_ENVIRONMENT_ID is not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { clusterEnvAccPreCheck(t) },
		ProtoV6ProviderFactories: clusterEnvAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "cycle_vpn" "test" {
  environment_id = %q
}

data "cycle_vpn_users" "test" {
  environment_id = %q
}
`, envID, envID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cycle_vpn.test", "environment_id", envID),
					resource.TestCheckResourceAttr("data.cycle_vpn.test", "id", envID),
					resource.TestCheckResourceAttrSet("data.cycle_vpn.test", "enable"),
					resource.TestCheckResourceAttr("data.cycle_vpn_users.test", "environment_id", envID),
					resource.TestCheckResourceAttrSet("data.cycle_vpn_users.test", "users.#"),
				),
			},
		},
	})
}
