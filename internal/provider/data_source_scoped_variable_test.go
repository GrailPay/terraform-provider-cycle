package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccScopedVariablesDataSource_list(t *testing.T) {
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
data "cycle_scoped_variables" "test" {
  environment_id = %q
}
`, envID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cycle_scoped_variables.test", "environment_id", envID),
					resource.TestCheckResourceAttrSet("data.cycle_scoped_variables.test", "variables.#"),
				),
			},
		},
	})
}

func TestAccScopedVariableDataSource_byIdentifier(t *testing.T) {
	envID := os.Getenv("CYCLE_ACC_ENVIRONMENT_ID")
	identifier := os.Getenv("CYCLE_ACC_SCOPED_VARIABLE_IDENTIFIER")
	if envID == "" || identifier == "" {
		t.Skip("CYCLE_ACC_ENVIRONMENT_ID and CYCLE_ACC_SCOPED_VARIABLE_IDENTIFIER must be set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { clusterEnvAccPreCheck(t) },
		ProtoV6ProviderFactories: clusterEnvAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "cycle_scoped_variable" "test" {
  environment_id = %q
  identifier     = %q
}
`, envID, identifier),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cycle_scoped_variable.test", "environment_id", envID),
					resource.TestCheckResourceAttr("data.cycle_scoped_variable.test", "identifier", identifier),
					resource.TestCheckResourceAttrSet("data.cycle_scoped_variable.test", "id"),
				),
			},
		},
	})
}
