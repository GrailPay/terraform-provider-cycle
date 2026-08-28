package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccHubMember_basic adopts an existing hub membership and manages its
// role. Requires CYCLE_TEST_MEMBER_ACCOUNT_ID to reference an account that
// is already a member of the test hub and is safe to re-role.
//
// NOTE: destroying this resource removes the member from the hub, so the
// referenced account will need to be re-invited after the test runs.
func TestAccHubMember_basic(t *testing.T) {
	accountID := os.Getenv("CYCLE_TEST_MEMBER_ACCOUNT_ID")
	if accountID == "" {
		t.Skip("CYCLE_TEST_MEMBER_ACCOUNT_ID not set; skipping hub member acceptance test")
	}

	config := fmt.Sprintf(`
resource "cycle_hub_role" "member_test" {
  identifier = "tf-acc-member-role"
  rank       = 1

  capabilities = {
    specific = ["environments-view"]
  }
}

resource "cycle_hub_member" "test" {
  account_id = %q
  role_id    = cycle_hub_role.member_test.id
}
`, accountID)

	updatedConfig := fmt.Sprintf(`
resource "cycle_hub_role" "member_test" {
  identifier = "tf-acc-member-role"
  rank       = 1

  capabilities = {
    specific = ["environments-view"]
  }
}

resource "cycle_hub_role" "member_test_2" {
  identifier = "tf-acc-member-role-2"
  rank       = 2

  capabilities = {
    specific = ["environments-view", "containers-view"]
  }
}

resource "cycle_hub_member" "test" {
  account_id = %q
  role_id    = cycle_hub_role.member_test_2.id
}
`, accountID)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { rolesUsersPreCheck(t) },
		ProtoV6ProviderFactories: rolesUsersProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cycle_hub_member.test", "id"),
					resource.TestCheckResourceAttr("cycle_hub_member.test", "account_id", accountID),
					resource.TestCheckResourceAttrSet("cycle_hub_member.test", "role_id"),
					resource.TestCheckResourceAttrSet("cycle_hub_member.test", "state"),
				),
			},
			{
				// Move the member to a different role in place.
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("cycle_hub_member.test", "role_id", "cycle_hub_role.member_test_2", "id"),
				),
			},
			{
				ResourceName:      "cycle_hub_member.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
