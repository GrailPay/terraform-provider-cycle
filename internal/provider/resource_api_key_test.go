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

var apiKeysAccProtoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"cycle": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func apiKeysAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("CYCLE_API_KEY") == "" {
		t.Fatal("CYCLE_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("CYCLE_HUB_ID") == "" {
		t.Fatal("CYCLE_HUB_ID must be set for acceptance tests")
	}
}

func TestAccAPIKeyResource_basic(t *testing.T) {
	roleID := os.Getenv("CYCLE_ACC_ROLE_ID")
	if roleID == "" {
		t.Skip("CYCLE_ACC_ROLE_ID must be set to run this acceptance test")
	}

	name := acctest.RandomWithPrefix("tf-acc-apikey")
	updated := name + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { apiKeysAccPreCheck(t) },
		ProtoV6ProviderFactories: apiKeysAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccAPIKeyResourceConfig(name, roleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_api_key.test", "name", name),
					resource.TestCheckResourceAttr("cycle_api_key.test", "role_id", roleID),
					resource.TestCheckResourceAttrSet("cycle_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_api_key.test", "secret"),
					resource.TestCheckResourceAttrSet("cycle_api_key.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_api_key.test", "state"),
				),
			},
			{
				ResourceName:            "cycle_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
			{
				Config: testAccAPIKeyResourceConfig(updated, roleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_api_key.test", "name", updated),
					resource.TestCheckResourceAttr("cycle_api_key.test", "role_id", roleID),
					resource.TestCheckResourceAttrSet("cycle_api_key.test", "secret"),
				),
			},
			{
				Config: testAccAPIKeyWithDataSourcesConfig(updated, roleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.cycle_api_keys.all", "api_keys.#"),
				),
			},
		},
	})
}

func testAccAPIKeyResourceConfig(name, roleID string) string {
	return fmt.Sprintf(`
resource "cycle_api_key" "test" {
  name    = %[1]q
  role_id = %[2]q
}
`, name, roleID)
}

func testAccAPIKeyWithDataSourcesConfig(name, roleID string) string {
	return fmt.Sprintf(`
resource "cycle_api_key" "test" {
  name    = %[1]q
  role_id = %[2]q
}

data "cycle_api_keys" "all" {}
`, name, roleID)
}
