<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/turbopuffer-lockup-dark.svg">
  <img alt="turbopuffer" src="assets/turbopuffer-lockup-light.svg" width="360">
</picture>

# Terraform Provider for turbopuffer

[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8.svg?logo=go)](go.mod)
[![Terraform Plugin Framework](https://img.shields.io/badge/plugin--framework-v1.19-844FBA.svg?logo=terraform)](https://developer.hashicorp.com/terraform/plugin/framework)

An unofficial [Terraform](https://www.terraform.io) provider for [turbopuffer](https://turbopuffer.com) — the fast, serverless vector database built on object storage. Manage namespaces, attribute schemas, and pinning declaratively, with drift detection and import support.

```hcl
resource "tpuf_namespace" "docs" {
  name = "docs"

  schema = {
    text = {
      type             = "string"
      full_text_search = true
    }
    embedding = {
      type = "[1536]f32"
      ann  = true
    }
    source = {
      type       = "string"
      filterable = true
    }
  }

  pinning_replicas = 1
}
```

- **Documentation**: [provider, resources & data sources](docs/)
- **Sister project**: [Steampipe plugin for turbopuffer](https://github.com/somoore/steampipe-plugin-turbopuffer) — query the same namespaces with SQL

## What it manages

| Name | Type | Description |
|---|---|---|
| `tpuf_namespace` | resource | Namespace lifecycle, attribute schema (filterable / full-text / glob / regex / ANN), pinning replicas |
| `tpuf_namespace` | data source | Read an existing namespace's schema and metadata |
| `tpuf_namespaces` | data source | List namespace names, optionally by prefix |

Document upserts and queries are data-plane operations that belong in application code — they are intentionally out of scope.

## Quick start

```hcl
terraform {
  required_providers {
    tpuf = {
      source = "somoore/tpuf"
    }
  }
}

provider "tpuf" {
  # Or set TURBOPUFFER_API_KEY / TURBOPUFFER_REGION environment variables.
  api_key = var.turbopuffer_api_key
  region  = "gcp-us-central1"
}
```

Import existing namespaces by name:

```sh
terraform import tpuf_namespace.docs docs
```

## Development

Requires [Go](https://golang.org/doc/install) 1.26+ and [Terraform](https://developer.hashicorp.com/terraform/downloads) 1.0+.

```sh
git clone https://github.com/somoore/terraform-provider-tpuf.git
cd terraform-provider-tpuf
make hooks   # activate pre-commit/pre-push checks (one time)
make build   # build the provider binary
make test    # gofmt + vet + security checks + all unit tests
make testacc # acceptance tests — needs TURBOPUFFER_API_KEY + TURBOPUFFER_REGION, creates real namespaces
make docs    # regenerate docs/ from schema + examples/
```

To run a local build, add a [development override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "somoore/tpuf" = "/path/to/terraform-provider-tpuf"
  }
  direct {}
}
```

## License

[Apache 2.0](LICENSE)

---

turbopuffer name, logo, and wordmark are trademarks of turbopuffer Inc. This is an unofficial, community-maintained provider and is not affiliated with or endorsed by turbopuffer Inc.
