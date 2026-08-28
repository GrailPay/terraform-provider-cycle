package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProviderLocationsDataSource(t *testing.T) {
	infraAccRequireEnv(t, "CYCLE_ACC_INTEGRATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "cycle_provider_locations" "all" {
  integration_id = %q
}
`, os.Getenv("CYCLE_ACC_INTEGRATION_ID")),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.cycle_provider_locations.all", "locations.#"),
				),
			},
		},
	})
}

func TestAccProviderServerModelsDataSource(t *testing.T) {
	infraAccRequireEnv(t, "CYCLE_ACC_INTEGRATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { infraAccPreCheck(t) },
		ProtoV6ProviderFactories: infraAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "cycle_provider_server_models" "all" {
  integration_id = %q
}
`, os.Getenv("CYCLE_ACC_INTEGRATION_ID")),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.cycle_provider_server_models.all", "models.#"),
				),
			},
		},
	})
}
