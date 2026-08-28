//go:build tools

package main

// Pin dependencies that resource/data source implementations and acceptance
// tests will import, so `go mod tidy` keeps them in go.mod before any code
// references them.
import (
	_ "github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	_ "github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	_ "github.com/hashicorp/terraform-plugin-log/tflog"
	_ "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)
