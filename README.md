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

## Installing

Signed release binaries for every OS/arch are published on the [GitHub releases page](https://github.com/somoore/terraform-provider-tpuf/releases). Each release includes a `SHA256SUMS` checksum file and a `SHA256SUMS.sig` GPG signature.

> This provider is **not currently on the Terraform Registry**, so `terraform init` won't fetch `somoore/tpuf` automatically. Install it from a release using one of the methods below. (If it is published later, the `required_providers` block in [Quick start](#quick-start) works as-is.)

### 1. Verify the release (recommended)

Download the release assets, then verify the archive checksums, and — if you have the signing key — the signature:

```sh
VERSION=0.1.0
OS=$(uname -s | tr A-Z a-z)
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')   # normalize to Terraform's arch names
ZIP="terraform-provider-tpuf_${VERSION}_${OS}_${ARCH}.zip"
BASE="https://github.com/somoore/terraform-provider-tpuf/releases/download/v${VERSION}"

# Grab the archive for your platform plus the checksum files.
curl -LO "${BASE}/${ZIP}"
curl -LO "${BASE}/terraform-provider-tpuf_${VERSION}_SHA256SUMS"
curl -LO "${BASE}/terraform-provider-tpuf_${VERSION}_SHA256SUMS.sig"

# Check the archive's SHA256 matches the published sum (prints "OK").
shasum -a 256 --ignore-missing -c "terraform-provider-tpuf_${VERSION}_SHA256SUMS"

# Optional: verify the checksum file itself is signed by the maintainer's key.
# Import the public key first (see releases page), then:
gpg --verify "terraform-provider-tpuf_${VERSION}_SHA256SUMS.sig" \
             "terraform-provider-tpuf_${VERSION}_SHA256SUMS"
```

On Linux, `sha256sum -c --ignore-missing …` is the equivalent of the `shasum` line.

### 2. Install via a filesystem mirror

Unzip the verified archive into a [filesystem mirror](https://developer.hashicorp.com/terraform/cli/config/config-file#filesystem_mirror) laid out by the registry's expected path, then point Terraform at it:

```sh
# Layout: <mirror>/<hostname>/<namespace>/<type>/<version>/<os>_<arch>/
DEST=~/.terraform.d/plugins/registry.terraform.io/somoore/tpuf/${VERSION}/${OS}_${ARCH}
mkdir -p "$DEST"
unzip -o "$ZIP" -d "$DEST"
```

```hcl
# ~/.terraformrc
provider_installation {
  filesystem_mirror {
    path    = "/Users/you/.terraform.d/plugins"
    include = ["registry.terraform.io/somoore/tpuf"]
  }
  direct {}   # everything else still resolves from the registry
}
```

`terraform init` will now resolve `somoore/tpuf` from the local mirror. (For iterating on the provider source instead, use the [dev override](#development) below.)

## Quick start

```hcl
terraform {
  required_providers {
    tpuf = {
      source  = "somoore/tpuf"
      version = "0.1.0"
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
