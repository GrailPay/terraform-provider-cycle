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

// containerAccProtoV6Factories is the provider factory map used by the
// cycle_container acceptance tests.
var containerAccProtoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"cycle": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func containerAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("CYCLE_API_KEY") == "" {
		t.Fatal("CYCLE_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("CYCLE_HUB_ID") == "" {
		t.Fatal("CYCLE_HUB_ID must be set for acceptance tests")
	}
}

func TestAccContainerResource_basic(t *testing.T) {
	envID := os.Getenv("CYCLE_ACC_ENVIRONMENT_ID")
	imageID := os.Getenv("CYCLE_ACC_IMAGE_ID")
	if envID == "" || imageID == "" {
		t.Skip("CYCLE_ACC_ENVIRONMENT_ID and CYCLE_ACC_IMAGE_ID must be set")
	}

	name := acctest.RandomWithPrefix("tf-acc-ctr")
	updated := name + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { containerAccPreCheck(t) },
		ProtoV6ProviderFactories: containerAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccContainerResourceConfig(name, envID, imageID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_container.test", "name", name),
					resource.TestCheckResourceAttr("cycle_container.test", "environment_id", envID),
					resource.TestCheckResourceAttr("cycle_container.test", "image_id", imageID),
					resource.TestCheckResourceAttr("cycle_container.test", "stateful", "false"),
					resource.TestCheckResourceAttr("cycle_container.test", "start_on_create", "false"),
					resource.TestCheckResourceAttrSet("cycle_container.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_container.test", "identifier"),
					resource.TestCheckResourceAttrSet("cycle_container.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_container.test", "state"),
					resource.TestCheckResourceAttrSet("cycle_container.test", "config"),
				),
			},
			{
				ResourceName:            "cycle_container.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"start_on_create", "config", "deployment"},
			},
			{
				Config: testAccContainerResourceConfig(updated, envID, imageID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_container.test", "name", updated),
				),
			},
			{
				Config: testAccContainerWithDataSourcesConfig(updated, envID, imageID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.cycle_container.by_id", "id", "cycle_container.test", "id"),
					resource.TestCheckResourceAttrPair("data.cycle_container.by_id", "name", "cycle_container.test", "name"),
					resource.TestCheckResourceAttrSet("data.cycle_containers.all", "containers.#"),
					resource.TestCheckResourceAttrSet("data.cycle_containers.by_env", "containers.#"),
				),
			},
		},
	})
}

// testAccContainerResourceConfig uses a conservative Cycle container config.
// cycle.Config requires Deploy (instances) and Network (hostname, public,
// egress_via_gateway). Public is disabled so the test does not bind host ports.
func testAccContainerResourceConfig(name, environmentID, imageID string) string {
	return fmt.Sprintf(`
resource "cycle_container" "test" {
  name           = %[1]q
  environment_id = %[2]q
  image_id       = %[3]q
  stateful       = false

  # cycle.Config requires deploy.instances and network.{hostname,public,egress_via_gateway}.
  config = jsonencode({
    deploy = {
      instances = 1
    }
    network = {
      hostname           = %[1]q
      public             = "disable"
      egress_via_gateway = false
    }
  })
}
`, name, environmentID, imageID)
}

func testAccContainerWithDataSourcesConfig(name, environmentID, imageID string) string {
	return testAccContainerResourceConfig(name, environmentID, imageID) + fmt.Sprintf(`
data "cycle_container" "by_id" {
  id = cycle_container.test.id
}

data "cycle_containers" "all" {}

data "cycle_containers" "by_env" {
  environment_id = %[1]q
}
`, environmentID)
}
