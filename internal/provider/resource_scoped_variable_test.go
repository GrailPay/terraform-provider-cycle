package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccScopedVariableResource_basic(t *testing.T) {
	clusterIdentifier := acctest.RandomWithPrefix("tf-acc-cluster")
	envName := acctest.RandomWithPrefix("tf-acc-env")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { clusterEnvAccPreCheck(t) },
		ProtoV6ProviderFactories: clusterEnvAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccScopedVariableResourceConfig(clusterIdentifier, envName, "API_TOKEN", "super-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_scoped_variable.test", "identifier", "API_TOKEN"),
					resource.TestCheckResourceAttr("cycle_scoped_variable.test", "value", "super-secret"),
					resource.TestCheckResourceAttr("cycle_scoped_variable.test", "scope.global", "true"),
					resource.TestCheckResourceAttr("cycle_scoped_variable.test", "access.env_variable.key", "API_TOKEN"),
					resource.TestCheckResourceAttrSet("cycle_scoped_variable.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_scoped_variable.test", "environment_id"),
				),
			},
			{
				ResourceName:      "cycle_scoped_variable.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Scoped variables import via the composite ID
				// "environment_id/scoped_variable_id".
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["cycle_scoped_variable.test"]
					if !ok {
						return "", fmt.Errorf("resource cycle_scoped_variable.test not found in state")
					}
					return rs.Primary.Attributes["environment_id"] + "/" + rs.Primary.ID, nil
				},
			},
			{
				Config: testAccScopedVariableResourceConfig(clusterIdentifier, envName, "API_TOKEN", "rotated-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_scoped_variable.test", "value", "rotated-secret"),
				),
			},
		},
	})
}

func testAccScopedVariableResourceConfig(clusterIdentifier, envName, identifier, value string) string {
	return fmt.Sprintf(`
resource "cycle_cluster" "test" {
  identifier = %[1]q
}

resource "cycle_environment" "test" {
  name    = %[2]q
  cluster = cycle_cluster.test.identifier
}

resource "cycle_scoped_variable" "test" {
  environment_id = cycle_environment.test.id
  identifier     = %[3]q
  value          = %[4]q

  access = {
    env_variable = {
      key = %[3]q
    }
  }
}
`, clusterIdentifier, envName, identifier, value)
}
