package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccHubWebhooksResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { servicesAccPreCheck(t) },
		ProtoV6ProviderFactories: servicesAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccHubWebhooksResourceConfig(
					"https://example.com/tf-acc/server-deployed",
					"https://example.com/tf-acc/server-deleted",
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_hub_webhooks.test", "id"),
					resource.TestCheckResourceAttr("cycle_hub_webhooks.test", "server_deployed", "https://example.com/tf-acc/server-deployed"),
					resource.TestCheckResourceAttr("cycle_hub_webhooks.test", "server_deleted", "https://example.com/tf-acc/server-deleted"),
				),
			},
			{
				ResourceName:      "cycle_hub_webhooks.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccHubWebhooksResourceConfig(
					"https://example.com/tf-acc/server-deployed-updated",
					"https://example.com/tf-acc/server-deleted-updated",
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_hub_webhooks.test", "server_deployed", "https://example.com/tf-acc/server-deployed-updated"),
					resource.TestCheckResourceAttr("cycle_hub_webhooks.test", "server_deleted", "https://example.com/tf-acc/server-deleted-updated"),
				),
			},
		},
	})
}

func testAccHubWebhooksResourceConfig(deployed, deleted string) string {
	return `
resource "cycle_hub_webhooks" "test" {
  server_deployed = "` + deployed + `"
  server_deleted  = "` + deleted + `"
}
`
}
