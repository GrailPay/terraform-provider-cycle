package provider_test

import (
	"os"
	"testing"

	"github.com/grailpay/terraform-provider-cycle/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// infraAccProtoV6Factories is the provider factory map shared by the
// infrastructure acceptance tests (servers, external volumes, auto-scale
// groups, and the related data sources).
var infraAccProtoV6Factories = map[string]func() (tfprotov6.ProviderServer, error){
	"cycle": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// infraAccPreCheck validates the credentials required by the infrastructure
// acceptance tests. Individual tests also skip when their extra CYCLE_ACC_*
// variables are unset.
func infraAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("CYCLE_API_KEY") == "" {
		t.Fatal("CYCLE_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("CYCLE_HUB_ID") == "" {
		t.Fatal("CYCLE_HUB_ID must be set for acceptance tests")
	}
}

func infraAccRequireEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if os.Getenv(key) == "" {
			t.Skipf("%s must be set to run this acceptance test", key)
		}
	}
}
