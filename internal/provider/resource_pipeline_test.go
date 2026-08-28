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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// pipelinesAccProtoV6Factories is the provider factory map shared by the
// pipeline and pipeline trigger key acceptance tests.
var pipelinesAccProtoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"cycle": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// pipelinesAccPreCheck validates the credentials required by the pipeline
// acceptance tests are present.
func pipelinesAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("CYCLE_API_KEY") == "" {
		t.Fatal("CYCLE_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("CYCLE_HUB_ID") == "" {
		t.Fatal("CYCLE_HUB_ID must be set for acceptance tests")
	}
}

func TestAccPipelineResource_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-pipeline")
	updated := name + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { pipelinesAccPreCheck(t) },
		ProtoV6ProviderFactories: pipelinesAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccPipelineResourceConfig(name, false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_pipeline.test", "name", name),
					resource.TestCheckResourceAttr("cycle_pipeline.test", "disable", "false"),
					resource.TestCheckResourceAttr("cycle_pipeline.test", "dynamic", "false"),
					resource.TestCheckResourceAttrSet("cycle_pipeline.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_pipeline.test", "identifier"),
					resource.TestCheckResourceAttrSet("cycle_pipeline.test", "hub_id"),
					resource.TestCheckResourceAttrSet("cycle_pipeline.test", "state"),
				),
			},
			{
				ResourceName:      "cycle_pipeline.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccPipelineResourceConfig(updated, true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_pipeline.test", "name", updated),
					resource.TestCheckResourceAttr("cycle_pipeline.test", "disable", "true"),
					resource.TestCheckResourceAttr("cycle_pipeline.test", "dynamic", "true"),
				),
			},
			{
				Config: testAccPipelineWithDataSourcesConfig(updated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.cycle_pipeline.by_id", "id", "cycle_pipeline.test", "id"),
					resource.TestCheckResourceAttrPair("data.cycle_pipeline.by_id", "name", "cycle_pipeline.test", "name"),
					resource.TestCheckResourceAttrSet("data.cycle_pipelines.all", "pipelines.#"),
				),
			},
		},
	})
}

func TestAccPipelineTriggerKeyResource_basic(t *testing.T) {
	pipelineName := acctest.RandomWithPrefix("tf-acc-pipeline")
	keyName := acctest.RandomWithPrefix("tf-acc-trigger")
	updatedKey := keyName + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { pipelinesAccPreCheck(t) },
		ProtoV6ProviderFactories: pipelinesAccProtoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: testAccPipelineTriggerKeyResourceConfig(pipelineName, keyName, "203.0.113.10"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_pipeline_trigger_key.test", "name", keyName),
					resource.TestCheckResourceAttr("cycle_pipeline_trigger_key.test", "ips.#", "1"),
					resource.TestCheckResourceAttr("cycle_pipeline_trigger_key.test", "ips.0", "203.0.113.10"),
					resource.TestCheckResourceAttrSet("cycle_pipeline_trigger_key.test", "id"),
					resource.TestCheckResourceAttrSet("cycle_pipeline_trigger_key.test", "secret"),
					resource.TestCheckResourceAttrSet("cycle_pipeline_trigger_key.test", "pipeline_id"),
				),
			},
			{
				ResourceName:            "cycle_pipeline_trigger_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["cycle_pipeline_trigger_key.test"]
					if !ok {
						return "", fmt.Errorf("resource cycle_pipeline_trigger_key.test not found in state")
					}
					return rs.Primary.Attributes["pipeline_id"] + "/" + rs.Primary.ID, nil
				},
			},
			{
				Config: testAccPipelineTriggerKeyResourceConfig(pipelineName, updatedKey, "203.0.113.20"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cycle_pipeline_trigger_key.test", "name", updatedKey),
					resource.TestCheckResourceAttr("cycle_pipeline_trigger_key.test", "ips.0", "203.0.113.20"),
					resource.TestCheckResourceAttrSet("cycle_pipeline_trigger_key.test", "secret"),
				),
			},
		},
	})
}

func testAccPipelineResourceConfig(name string, disable, dynamic bool) string {
	return fmt.Sprintf(`
resource "cycle_pipeline" "test" {
  name    = %[1]q
  disable = %[2]t
  dynamic = %[3]t
}
`, name, disable, dynamic)
}

func testAccPipelineWithDataSourcesConfig(name string) string {
	return fmt.Sprintf(`
resource "cycle_pipeline" "test" {
  name    = %[1]q
  disable = true
  dynamic = true
}

data "cycle_pipeline" "by_id" {
  id = cycle_pipeline.test.id
}

data "cycle_pipelines" "all" {}
`, name)
}

func testAccPipelineTriggerKeyResourceConfig(pipelineName, keyName, ip string) string {
	return fmt.Sprintf(`
resource "cycle_pipeline" "test" {
  name = %[1]q
}

resource "cycle_pipeline_trigger_key" "test" {
  pipeline_id = cycle_pipeline.test.id
  name        = %[2]q
  ips         = [%[3]q]
}
`, pipelineName, keyName, ip)
}
