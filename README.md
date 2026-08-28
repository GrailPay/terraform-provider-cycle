# Terraform Provider for Cycle.io

A Terraform provider for [Cycle.io](https://cycle.io), the LowOps platform for containers and infrastructure. Manage clusters, environments, scoped variables, DNS, image sources, hub membership, environment services, infrastructure, pipelines, and stacks as code.

Built with the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework) (protocol version 6) on top of Cycle's official [Go API client](https://github.com/cycleplatform/api-client-go).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0 (plugin protocol 6)
- [Go](https://go.dev/doc/install) >= 1.26 (to build from source)
- A Cycle hub and an API key with appropriate permissions

## Usage

```hcl
terraform {
  required_providers {
    cycle = {
      source  = "grailpay/cycle"
      version = "~> 0.1"
    }
  }
}

provider "cycle" {
  # Both may be omitted and provided via the CYCLE_API_KEY and
  # CYCLE_HUB_ID environment variables instead.
  api_key = var.cycle_api_key
  hub_id  = var.cycle_hub_id
}

resource "cycle_environment" "production" {
  name        = "Production"
  cluster     = "production"
  description = "Managed by Terraform"
}
```

Provider configuration:

| Attribute | Environment variable | Description |
|-----------|----------------------|-------------|
| `api_key` | `CYCLE_API_KEY` | Cycle API key (sensitive) |
| `hub_id`  | `CYCLE_HUB_ID`  | ID of the hub to manage |
| `api_url` | —               | API base URL, defaults to `https://api.cycle.io` |

## Resources

- `cycle_cluster` — infrastructure clusters
- `cycle_environment` — environments within a cluster
- `cycle_scoped_variable` — environment scoped variables (env var / internal API / file access, secret values)
- `cycle_dns_zone` — hosted or linked DNS zones
- `cycle_dns_record` — records within a DNS zone (A, AAAA, CNAME, TXT, MX, LINKED, ...)
- `cycle_image_source` — image sources (Docker Hub, registries, OCI, stack builds)
- `cycle_hub_role` — custom hub roles with capabilities
- `cycle_hub_invite` — invite users to the hub by email
- `cycle_hub_member` — manage an existing hub member's role
- `cycle_hub_webhooks` — hub `server_deployed` / `server_deleted` webhook URLs
- `cycle_load_balancer` — environment load balancer service (reconfigure singleton)
- `cycle_vpn` — environment VPN service (reconfigure singleton)
- `cycle_vpn_user` — VPN accounts for an environment
- `cycle_server` — provision a server into a cluster
- `cycle_external_volume` — external volumes
- `cycle_autoscale_group` — auto-scale groups
- `cycle_pipeline` — pipelines (`stages` as JSON)
- `cycle_pipeline_trigger_key` — pipeline trigger keys (secret is computed + sensitive)
- `cycle_stack` — stacks from a git repo or raw spec
- `cycle_stack_build` — create a stack build and wait until it is live

## Data Sources

- `cycle_hub` — the current hub
- `cycle_cluster`, `cycle_environment`, `cycle_environments`
- `cycle_dns_zone`
- `cycle_image`, `cycle_images`, `cycle_image_source`
- `cycle_hub_roles`, `cycle_hub_members`
- `cycle_load_balancer`
- `cycle_server`, `cycle_servers`
- `cycle_ip_pool`, `cycle_ip_pools`
- `cycle_external_volume`, `cycle_autoscale_group`
- `cycle_provider_locations`, `cycle_provider_server_models`
- `cycle_pipeline`, `cycle_pipelines`
- `cycle_stack`, `cycle_stacks`

Full documentation for every resource and data source lives in [`docs/`](docs/) and is rendered on the Terraform Registry once published.

## Building Locally

```sh
make build          # builds ./terraform-provider-cycle
make install        # go install into $GOPATH/bin
```

To test a local build without publishing, add a `dev_overrides` block to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "grailpay/cycle" = "/path/to/your/go/bin" # output of: go env GOPATH, plus /bin
  }
  direct {}
}
```

Then run `go install .` and use the provider in any Terraform configuration — Terraform will print a warning that the override is in effect. Skip `terraform init` for the overridden provider; `terraform plan`/`apply` work directly.

## Running Tests

Unit tests (no credentials required):

```sh
make test
```

Acceptance tests run against a **real Cycle hub and create, modify, and destroy real infrastructure** (clusters, environments, DNS zones, invites, etc.). Costs may apply. Use a dedicated test hub, not production:

```sh
export CYCLE_API_KEY="your-api-key"
export CYCLE_HUB_ID="your-hub-id"
make testacc        # runs: TF_ACC=1 go test ./... -v -timeout 120m
```

## Regenerating Documentation

Docs in `docs/` are generated from the provider schema and the examples in `examples/` using [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs):

```sh
make docs
```

## Publishing to the Terraform Registry

One-time setup, then every release is just a git tag.

1. **Create the GitHub repo** `grailpay/terraform-provider-cycle` with `master` as the default branch, and push this repo to it:

   ```sh
   git remote add origin git@github.com:grailpay/terraform-provider-cycle.git
   git push -u origin master
   ```

2. **Create a GPG signing key** (if you don't already have one) and export it:

   ```sh
   gpg --full-generate-key            # RSA, no expiry is fine
   gpg --armor --export-secret-keys KEY_ID   # private key, for GitHub secret
   gpg --armor --export KEY_ID               # public key, for the registry
   ```

3. **Add repo secrets** (GitHub → Settings → Secrets and variables → Actions):
   - `GPG_PRIVATE_KEY` — the ASCII-armored private key
   - `PASSPHRASE` — the key's passphrase

4. **Tag a release.** The [release workflow](.github/workflows/release.yml) runs GoReleaser, which builds multi-platform binaries, signs the checksums, and creates a GitHub release:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

5. **Publish on the registry** (first release only): sign in to [registry.terraform.io](https://registry.terraform.io) with GitHub, go to *Publish → Provider*, select `grailpay/terraform-provider-cycle`, and upload the GPG **public** key. The registry ingests the tagged release automatically; subsequent tags appear without further manual steps.

> [!NOTE]
> The registry requires the repository to contain an open source license file (most providers use MPL-2.0). Add a `LICENSE` file before publishing if one is not present.
