package provider_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/grailpay/terraform-provider-cycle/internal/provider"
)

// rolesUsersProviderFactories is the provider factory map shared by the
// roles/users acceptance tests in this package.
var rolesUsersProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"cycle": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// rolesUsersPreCheck validates the environment for the roles/users
// acceptance tests. Tests are already gated on TF_ACC by resource.Test.
func rolesUsersPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("CYCLE_API_KEY") == "" {
		t.Fatal("CYCLE_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("CYCLE_HUB_ID") == "" {
		t.Fatal("CYCLE_HUB_ID must be set for acceptance tests")
	}
}

func TestAccHubRole_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { rolesUsersPreCheck(t) },
		ProtoV6ProviderFactories: rolesUsersProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "cycle_hub_role" "test" {
  name       = "TF Acc Test Role"
  identifier = "tf-acc-test-role"
  rank       = 2

  capabilities = {
    specific = [
      "environments-view",
      "containers-view",
    ]
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_hub_role.test", "id"),
					resource.TestCheckResourceAttr("cycle_hub_role.test", "name", "TF Acc Test Role"),
					resource.TestCheckResourceAttr("cycle_hub_role.test", "identifier", "tf-acc-test-role"),
					resource.TestCheckResourceAttr("cycle_hub_role.test", "rank", "2"),
					resource.TestCheckResourceAttr("cycle_hub_role.test", "root", "false"),
					resource.TestCheckResourceAttr("cycle_hub_role.test", "capabilities.all", "false"),
					resource.TestCheckResourceAttr("cycle_hub_role.test", "capabilities.specific.#", "2"),
				),
			},
			{
				// Update rank and switch to the "all" capabilities flag.
				Config: `
resource "cycle_hub_role" "test" {
  name       = "TF Acc Test Role Updated"
  identifier = "tf-acc-test-role"
  rank       = 3

  capabilities = {
    all = true
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_hub_role.test", "name", "TF Acc Test Role Updated"),
					resource.TestCheckResourceAttr("cycle_hub_role.test", "rank", "3"),
					resource.TestCheckResourceAttr("cycle_hub_role.test", "capabilities.all", "true"),
				),
			},
			{
				ResourceName:      "cycle_hub_role.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
