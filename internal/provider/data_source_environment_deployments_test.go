package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnvironmentDeploymentsDataSource_basic(t *testing.T) {
	envID := os.Getenv("CYCLE_ACC_ENVIRONMENT_ID")
	if envID == "" {
		t.Skip("CYCLE_ACC_ENVIRONMENT_ID is not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { clusterEnvAccPreCheck(t) },
		ProtoV6ProviderFactories: clusterEnvAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentDeploymentsDataSourceConfig(envID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cycle_environment_deployments.test", "environment_id", envID),
					resource.TestCheckResourceAttr("data.cycle_environment_deployments.test", "id", envID),
					resource.TestCheckResourceAttrSet("data.cycle_environment_deployments.test", "versions.#"),
				),
			},
		},
	})
}

func testAccEnvironmentDeploymentsDataSourceConfig(environmentID string) string {
	return fmt.Sprintf(`
data "cycle_environment_deployments" "test" {
  environment_id = %q
}
`, environmentID)
}
