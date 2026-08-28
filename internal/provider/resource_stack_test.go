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

// stacksAccProtoV6Factories is the provider factory map shared by the stack
// acceptance tests.
var stacksAccProtoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"cycle": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// stacksAccPreCheck validates the credentials required by the stack
// acceptance tests are present.
func stacksAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("CYCLE_API_KEY") == "" {
		t.Fatal("CYCLE_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("CYCLE_HUB_ID") == "" {
		t.Fatal("CYCLE_HUB_ID must be set for acceptance tests")
	}
}

func TestAccStackResource_raw(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-stack")
	updated := name + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { stacksAccPreCheck(t) },
		ProtoV6ProviderFactories: stacksAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccStackResourceRawConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_stack.test", "name", name),
					resource.TestCheckResourceAttrSet("cycle_stack.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_stack.test", "identifier"),
					resource.TestCheckResourceAttrSet("cycle_stack.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_stack.test", "state"),
					resource.TestCheckResourceAttrSet("cycle_stack.test", "source.raw"),
				),
			},
			{
				ResourceName:      "cycle_stack.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccStackResourceRawConfig(updated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_stack.test", "name", updated),
				),
			},
			{
				Config: testAccStackWithDataSourcesConfig(updated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.cycle_stack.by_id", "id", "cycle_stack.test", "id"),
					resource.TestCheckResourceAttrPair("data.cycle_stack.by_identifier", "identifier", "cycle_stack.test", "identifier"),
					resource.TestCheckResourceAttrSet("data.cycle_stacks.all", "stacks.#"),
				),
			},
		},
	})
}

func testAccStackRawSpecJSON() string {
	// Minimal Cycle stack spec: version 1.0 plus one container sourced from
	// Docker Hub. Creating a stack stores the spec; it does not build images.
	return `{
  version = "1.0"
  about = {
    description = "tf acceptance test stack"
  }
  containers = {
    web = {
      name = "web"
      image = {
        name = "nginx"
        origin = {
          type = "docker-hub"
          details = {
            target = "nginx:alpine"
          }
        }
      }
    }
  }
}`
}

func testAccStackResourceRawConfig(name string) string {
	return fmt.Sprintf(`
resource "cycle_stack" "test" {
  name = %[1]q

  source = {
    raw = jsonencode(%s)
  }
}
`, name, testAccStackRawSpecJSON())
}

func testAccStackWithDataSourcesConfig(name string) string {
	return fmt.Sprintf(`
resource "cycle_stack" "test" {
  name = %[1]q

  source = {
    raw = jsonencode(%s)
  }
}

data "cycle_stack" "by_id" {
  id = cycle_stack.test.id
}

data "cycle_stack" "by_identifier" {
  identifier = cycle_stack.test.identifier
}

data "cycle_stacks" "all" {}
`, name, testAccStackRawSpecJSON())
}
