package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/grailpay/terraform-provider-cycle/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var integrationsAccProtoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"cycle": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func integrationsAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("CYCLE_API_KEY") == "" {
		t.Fatal("CYCLE_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("CYCLE_HUB_ID") == "" {
		t.Fatal("CYCLE_HUB_ID must be set for acceptance tests")
	}
}

func TestAccIntegrationResource_basic(t *testing.T) {
	vendor := os.Getenv("CYCLE_ACC_INTEGRATION_VENDOR")
	if vendor == "" {
		t.Skip("CYCLE_ACC_INTEGRATION_VENDOR must be set to run this acceptance test")
	}

	name := acctest.RandomWithPrefix("tf-acc-integration")
	updated := name + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { integrationsAccPreCheck(t) },
		ProtoV6ProviderFactories: integrationsAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccIntegrationResourceConfig(name, vendor),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_integration.test", "name", name),
					resource.TestCheckResourceAttr("cycle_integration.test", "vendor", vendor),
					resource.TestCheckResourceAttrSet("cycle_integration.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_integration.test", "identifier"),
					resource.TestCheckResourceAttrSet("cycle_integration.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_integration.test", "state"),
				),
			},
			{
				ResourceName:      "cycle_integration.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccIntegrationResourceConfig(updated, vendor),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_integration.test", "name", updated),
					resource.TestCheckResourceAttr("cycle_integration.test", "vendor", vendor),
				),
			},
			{
				Config: testAccIntegrationWithDataSourcesConfig(updated, vendor),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.cycle_integration.by_id", "id", "cycle_integration.test", "id"),
					resource.TestCheckResourceAttrPair("data.cycle_integration.by_id", "name", "cycle_integration.test", "name"),
					resource.TestCheckResourceAttrSet("data.cycle_integrations.all", "integrations.#"),
				),
			},
		},
	})
}

func TestAccAvailableIntegrationsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { integrationsAccPreCheck(t) },
		ProtoV6ProviderFactories: integrationsAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: `data "cycle_available_integrations" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.cycle_available_integrations.all", "integrations.#"),
					resource.TestCheckResourceAttrSet("data.cycle_available_integrations.all", "integrations.0.category"),
					resource.TestCheckResourceAttrSet("data.cycle_available_integrations.all", "integrations.0.vendor"),
					resource.TestCheckResourceAttrSet("data.cycle_available_integrations.all", "integrations.0.name"),
				),
			},
		},
	})
}

func testAccIntegrationResourceConfig(name, vendor string) string {
	return fmt.Sprintf(`
resource "cycle_integration" "test" {
  name   = %[1]q
  vendor = %[2]q
}
`, name, vendor)
}

func testAccIntegrationWithDataSourcesConfig(name, vendor string) string {
	return fmt.Sprintf(`
resource "cycle_integration" "test" {
  name   = %[1]q
  vendor = %[2]q
}

data "cycle_integration" "by_id" {
  id = cycle_integration.test.id
}

data "cycle_integrations" "all" {}
`, name, vendor)
}
