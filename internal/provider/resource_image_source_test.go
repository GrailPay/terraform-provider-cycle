package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccImageSourceResource(t *testing.T) {
	name := fmt.Sprintf("tf-acc-%s", acctest.RandString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { dnsImagesPreCheck(t) },
		ProtoV6ProviderFactories: dnsImagesProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccImageSourceConfig(name, "An acceptance test image source."),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_image_source.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_image_source.test", "identifier"),
					resource.TestCheckResourceAttr("cycle_image_source.test", "name", name),
					resource.TestCheckResourceAttr("cycle_image_source.test", "type", "direct"),
					resource.TestCheckResourceAttr("cycle_image_source.test", "description", "An acceptance test image source."),
					resource.TestCheckResourceAttr("cycle_image_source.test", "origin.docker_hub.target", "traefik/whoami:latest"),
				),
			},
			{
				// Change the description to exercise Update.
				Config: testAccImageSourceConfig(name, "Updated description."),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_image_source.test", "description", "Updated description."),
				),
			},
			{
				ResourceName:            "cycle_image_source.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"state"},
			},
		},
	})
}

func TestAccImageSourceDataSource(t *testing.T) {
	name := fmt.Sprintf("tf-acc-%s", acctest.RandString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { dnsImagesPreCheck(t) },
		ProtoV6ProviderFactories: dnsImagesProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccImageSourceConfig(name, "An acceptance test image source.") + `
data "cycle_image_source" "by_id" {
  id = cycle_image_source.test.id
}

data "cycle_image_source" "by_identifier" {
  identifier = cycle_image_source.test.identifier
}

data "cycle_images" "from_source" {
  source_id = cycle_image_source.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.cycle_image_source.by_id", "name", "cycle_image_source.test", "name"),
					resource.TestCheckResourceAttrPair("data.cycle_image_source.by_identifier", "id", "cycle_image_source.test", "id"),
					resource.TestCheckResourceAttrSet("data.cycle_images.from_source", "images.#"),
				),
			},
		},
	})
}

func testAccImageSourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "cycle_image_source" "test" {
  name        = %[1]q
  type        = "direct"
  description = %[2]q

  origin = {
    docker_hub = {
      target = "traefik/whoami:latest"
    }
  }
}
`, name, description)
}
