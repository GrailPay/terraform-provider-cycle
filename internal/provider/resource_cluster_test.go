package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/tomschlick/terraform-provider-cycle/internal/provider"
)

// clusterEnvAccProtoV6Factories is the provider factory map shared by the
// cluster, environment, and scoped variable acceptance tests.
var clusterEnvAccProtoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"cycle": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// clusterEnvAccPreCheck validates the credentials required by the cluster,
// environment, and scoped variable acceptance tests are present.
func clusterEnvAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("CYCLE_API_KEY") == "" {
		t.Fatal("CYCLE_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("CYCLE_HUB_ID") == "" {
		t.Fatal("CYCLE_HUB_ID must be set for acceptance tests")
	}
}

func TestAccClusterResource_basic(t *testing.T) {
	identifier := acctest.RandomWithPrefix("tf-acc-cluster")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { clusterEnvAccPreCheck(t) },
		ProtoV6ProviderFactories: clusterEnvAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccClusterResourceConfig(identifier, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_cluster.test", "identifier", identifier),
					resource.TestCheckResourceAttr("cycle_cluster.test", "non_essential", "false"),
					resource.TestCheckResourceAttrSet("cycle_cluster.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_cluster.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_cluster.test", "state"),
				),
			},
			{
				ResourceName:      "cycle_cluster.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccClusterResourceConfig(identifier, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_cluster.test", "non_essential", "true"),
				),
			},
		},
	})
}

func testAccClusterResourceConfig(identifier string, nonEssential bool) string {
	return fmt.Sprintf(`
resource "cycle_cluster" "test" {
  identifier    = %[1]q
  non_essential = %[2]t
}
`, identifier, nonEssential)
}
