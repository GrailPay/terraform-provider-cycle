package provider_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/grailpay/terraform-provider-cycle/internal/provider"
)

// TestProviderSchema starts the provider server in-process and asks it for
// its schema, which exercises schema validation for the provider and all
// registered resources/data sources.
func TestProviderSchema(t *testing.T) {
	srv := providerserver.NewProtocol6(provider.New("test")())()

	resp, err := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema returned error: %v", err)
	}

	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema diagnostic error: %s: %s", d.Summary, d.Detail)
		}
	}

	if resp.Provider == nil {
		t.Fatal("expected a provider schema, got nil")
	}
}

// TestProviderRegistrations asserts that every expected resource and data
// source type is registered with the provider, catching accidentally dropped
// init() registrations.
func TestProviderRegistrations(t *testing.T) {
	srv := providerserver.NewProtocol6(provider.New("test")())()

	resp, err := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema returned error: %v", err)
	}

	wantResources := []string{
		"cycle_cluster",
		"cycle_dns_record",
		"cycle_dns_zone",
		"cycle_environment",
		"cycle_hub_invite",
		"cycle_hub_member",
		"cycle_hub_role",
		"cycle_image_source",
		"cycle_scoped_variable",
	}
	wantDataSources := []string{
		"cycle_cluster",
		"cycle_dns_zone",
		"cycle_environment",
		"cycle_environments",
		"cycle_hub",
		"cycle_hub_members",
		"cycle_hub_roles",
		"cycle_image",
		"cycle_image_source",
		"cycle_images",
	}

	for _, name := range wantResources {
		if _, ok := resp.ResourceSchemas[name]; !ok {
			t.Errorf("resource %q is not registered", name)
		}
	}
	if got, want := len(resp.ResourceSchemas), len(wantResources); got != want {
		t.Errorf("registered resource count = %d, want %d (schemas: %v)", got, want, keys(resp.ResourceSchemas))
	}

	for _, name := range wantDataSources {
		if _, ok := resp.DataSourceSchemas[name]; !ok {
			t.Errorf("data source %q is not registered", name)
		}
	}
	if got, want := len(resp.DataSourceSchemas), len(wantDataSources); got != want {
		t.Errorf("registered data source count = %d, want %d (schemas: %v)", got, want, keys(resp.DataSourceSchemas))
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
