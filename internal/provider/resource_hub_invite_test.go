package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccHubInvite_basic invites an email address to the hub, then revokes
// the invite on destroy. Requires CYCLE_TEST_INVITE_EMAIL to point at an
// address that is safe to invite (it will receive a real invitation email).
func TestAccHubInvite_basic(t *testing.T) {
	email := os.Getenv("CYCLE_TEST_INVITE_EMAIL")
	if email == "" {
		t.Skip("CYCLE_TEST_INVITE_EMAIL not set; skipping hub invite acceptance test")
	}

	config := fmt.Sprintf(`
resource "cycle_hub_role" "invite_test" {
  identifier = "tf-acc-invite-role"
  rank       = 1

  capabilities = {
    specific = ["environments-view"]
  }
}

resource "cycle_hub_invite" "test" {
  recipient = %q
  role_id   = cycle_hub_role.invite_test.id
}
`, email)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { rolesUsersPreCheck(t) },
		ProtoV6ProviderFactories: rolesUsersProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_hub_invite.test", "id"),
					resource.TestCheckResourceAttr("cycle_hub_invite.test", "recipient", email),
					resource.TestCheckResourceAttrSet("cycle_hub_invite.test", "role_id"),
					resource.TestCheckResourceAttr("cycle_hub_invite.test", "state", "pending"),
				),
			},
			{
				ResourceName:      "cycle_hub_invite.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
