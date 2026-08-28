package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccHubDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { rolesUsersPreCheck(t) },
		ProtoV6ProviderFactories: rolesUsersProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "cycle_hub" "current" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.cycle_hub.current", "id"),
					resource.TestCheckResourceAttrSet("data.cycle_hub.current", "name"),
					resource.TestCheckResourceAttrSet("data.cycle_hub.current", "identifier"),
					resource.TestCheckResourceAttrSet("data.cycle_hub.current", "state"),
				),
			},
		},
	})
}

func TestAccHubRolesDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { rolesUsersPreCheck(t) },
		ProtoV6ProviderFactories: rolesUsersProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "cycle_hub_roles" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Every hub has at least the built-in owner role.
					resource.TestCheckResourceAttrSet("data.cycle_hub_roles.all", "roles.0.id"),
					resource.TestCheckResourceAttrSet("data.cycle_hub_roles.all", "roles.0.identifier"),
				),
			},
		},
	})
}

func TestAccHubMembersDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { rolesUsersPreCheck(t) },
		ProtoV6ProviderFactories: rolesUsersProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "cycle_hub_members" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Every hub has at least its owner as a member.
					resource.TestCheckResourceAttrSet("data.cycle_hub_members.all", "members.0.id"),
					resource.TestCheckResourceAttrSet("data.cycle_hub_members.all", "members.0.role_id"),
				),
			},
		},
	})
}
