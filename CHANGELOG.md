# Changelog

## 0.1.1 (Unreleased)

When Cycle deletes a finished job, `GET /v1/jobs/{id}` starts 404ing. Create
now treats that as success and resolves the new server from the cluster list
(matching location, model, and zone). Other resources treat a missing job as
completed instead of failing the apply.

## 0.1.0 (Unreleased)

Initial release.

Added load balancer, VPN, VPN users, hub webhooks, servers, external volumes, auto-scale groups, IP pool data sources, pipelines, trigger keys, stacks, and stack builds.

Added hub integrations, hub API keys, containers, environment discovery/gateway/scheduler services, SDN networks, and user-supplied TLS certificates.

Added `cycle_environment_deployments` data source for environment deployment versions and tags.

Added data sources for scoped variables, stack builds, VPN, VPN users, DNS zones/records, pool IPs, clusters, and image sources.
