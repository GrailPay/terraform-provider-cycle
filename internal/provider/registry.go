package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Registration convention
//
// Every resource and data source lives in its own file in this package
// (e.g. resource_environment.go, data_source_environment.go) and registers
// itself from an init() func:
//
//	func init() {
//		RegisterResource(NewEnvironmentResource)
//	}
//
// The provider's Resources()/DataSources() methods return the accumulated
// factories. This means adding a new resource or data source never requires
// editing a shared file, so parallel work cannot produce merge conflicts here.

var resourceFactories []func() resource.Resource

var dataSourceFactories []func() datasource.DataSource

// RegisterResource adds a resource factory to the provider. Call from init().
func RegisterResource(f func() resource.Resource) {
	resourceFactories = append(resourceFactories, f)
}

// RegisterDataSource adds a data source factory to the provider. Call from init().
func RegisterDataSource(f func() datasource.DataSource) {
	dataSourceFactories = append(dataSourceFactories, f)
}
