package provider

import (
	"fmt"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// CycleClient is the shared provider data handed to every resource and data
// source via Configure. It wraps the generated oapi-codegen client plus the
// hub ID the provider was configured with.
type CycleClient struct {
	// Client is the authenticated Cycle API client. All generated call
	// methods follow the oapi-codegen "WithResponse" style, e.g.
	// Client.GetEnvironmentsWithResponse(ctx, &cycle.GetEnvironmentsParams{}).
	Client *cycle.ClientWithResponses

	// HubID is the hub the provider is operating on. The client already
	// injects it as the X-Hub-Id header on every request; it is exposed here
	// for resources that need it in request bodies or composite IDs.
	HubID string
}

// clientFromResourceConfigure extracts the *CycleClient from a resource
// ConfigureRequest. Returns nil (with a diagnostic added) when provider data
// is missing or of an unexpected type. A nil ProviderData (Terraform calls
// Configure before the provider itself is configured, e.g. during validate)
// returns nil without diagnostics; callers should treat nil as "not yet
// configured" and return early.
func clientFromResourceConfigure(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *CycleClient {
	if req.ProviderData == nil {
		return nil
	}

	c, ok := req.ProviderData.(*CycleClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.CycleClient, got %T. This is a bug in the provider.", req.ProviderData),
		)
		return nil
	}

	return c
}

// clientFromDataSourceConfigure is the data source counterpart of
// clientFromResourceConfigure.
func clientFromDataSourceConfigure(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *CycleClient {
	if req.ProviderData == nil {
		return nil
	}

	c, ok := req.ProviderData.(*CycleClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *provider.CycleClient, got %T. This is a bug in the provider.", req.ProviderData),
		)
		return nil
	}

	return c
}
