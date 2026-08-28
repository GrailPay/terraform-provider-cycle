package provider

import (
	"fmt"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// apiError converts a Cycle API error envelope (the JSONDefault field on
// every generated *Response struct; cycle.DefaultError is an alias of
// cycle.ErrorEnvelope) into a readable Go error.
//
// action should describe the attempted operation, e.g. "creating environment".
func apiError(action string, statusCode int, envelope *cycle.ErrorEnvelope) error {
	if envelope == nil {
		return fmt.Errorf("%s: Cycle API returned unexpected status %d with no error body", action, statusCode)
	}

	msg := "unknown error"
	if envelope.Error.Title != nil && *envelope.Error.Title != "" {
		msg = *envelope.Error.Title
	}
	if envelope.Error.Detail != nil && *envelope.Error.Detail != "" {
		msg += ": " + *envelope.Error.Detail
	}
	if envelope.Error.Code != nil && *envelope.Error.Code != "" {
		msg += fmt.Sprintf(" (code: %s)", *envelope.Error.Code)
	}

	return fmt.Errorf("%s: Cycle API error (HTTP %d): %s", action, statusCode, msg)
}

// addAPIError is a convenience for resources/data sources: it converts an
// API error envelope into a diagnostic on diags.
//
//	if resp.JSON200 == nil {
//		addAPIError(&res.Diagnostics, "creating environment", resp.StatusCode(), resp.JSONDefault)
//		return
//	}
func addAPIError(diags *diag.Diagnostics, action string, statusCode int, envelope *cycle.ErrorEnvelope) {
	diags.AddError(
		fmt.Sprintf("Error %s", action),
		apiError(action, statusCode, envelope).Error(),
	)
}
