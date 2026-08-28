package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccStackBuildsDataSource_list(t *testing.T) {
	stackID := os.Getenv("CYCLE_ACC_STACK_ID")
	if stackID == "" {
		t.Skip("CYCLE_ACC_STACK_ID is not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { stacksAccPreCheck(t) },
		ProtoV6ProviderFactories: stacksAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "cycle_stack_builds" "test" {
  stack_id = %q
}
`, stackID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cycle_stack_builds.test", "stack_id", stackID),
					resource.TestCheckResourceAttrSet("data.cycle_stack_builds.test", "builds.#"),
				),
			},
		},
	})
}

func TestAccStackBuildDataSource_byID(t *testing.T) {
	stackID := os.Getenv("CYCLE_ACC_STACK_ID")
	buildID := os.Getenv("CYCLE_ACC_STACK_BUILD_ID")
	if stackID == "" || buildID == "" {
		t.Skip("CYCLE_ACC_STACK_ID and CYCLE_ACC_STACK_BUILD_ID must be set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { stacksAccPreCheck(t) },
		ProtoV6ProviderFactories: stacksAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "cycle_stack_build" "test" {
  stack_id = %q
  id       = %q
}
`, stackID, buildID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cycle_stack_build.test", "stack_id", stackID),
					resource.TestCheckResourceAttr("data.cycle_stack_build.test", "id", buildID),
					resource.TestCheckResourceAttrSet("data.cycle_stack_build.test", "state"),
				),
			},
		},
	})
}
