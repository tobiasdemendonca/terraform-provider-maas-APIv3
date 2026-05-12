# AGENTS.md

Practical guide for AI coding agents working on this repository. Read this first, then see [CLAUDE.md](CLAUDE.md) for the full product spec and requirements.

## Project

Terraform provider for the MAAS API v3, built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework). The Go client is auto-generated from the OpenAPI spec via `oapi-codegen`.

Module: `terraform-provider-maas-apiv3`

## Essential Commands

```sh
make build            # compile
make install          # build and install binary to $GOPATH/bin
make generate         # run go generate (docs, etc.)
make generate-client  # convert api/openapi.json → api/openapi.converted.json, then run oapi-codegen
make fmt              # gofmt
make lint             # golangci-lint
make test             # unit tests
make testacc          # acceptance tests (requires live MAAS + TF_ACC=1)
make create-dev-overrides  # write dev.tfrc for local provider testing
```

## Project Layout

```
api/
  openapi.json              # upstream MAAS OpenAPI spec (source of truth, do not edit)
  openapi.converted.json    # generated — gitignored, produced by make generate-client
docs/
  ADRs/                     # Architecture Decision Records (NNNN-title.md)
internal/
  client/
    maasclientv3/
      oapi-codegen.yaml     # codegen config
      client.gen.go         # generated — do not edit by hand
  provider/                 # Terraform provider resources, data sources, functions
scripts/
  fix-openapi-nullable.py   # converts 3.1 nullable syntax to 3.0 for oapi-codegen
tools/
  tools.go                  # go:generate directives for tooling
GNUmakefile
```

## Conventions

- **Framework**: always use `terraform-plugin-framework`. Imports from `terraform-plugin-sdk/v2` are banned by the linter.
- **Generated code**: `client.gen.go` and anything under `*.gen.go` is machine-generated — fix issues upstream (spec or config), not in the generated file.
- **OpenAPI spec fixes**: if `api/openapi.json` has issues that block codegen, add a transformation to `scripts/fix-openapi-nullable.py` rather than editing the spec directly.
- **ADRs**: document significant architectural decisions in `docs/ADRs/` using the `NNNN-title.md` naming convention.
- **No global state**: the provider must support aliases for multi-MAAS deployments — avoid package-level variables.

## Key Constraints

- Acceptance tests must be idempotent (no trailing resources, no changed config values).
- Resources must handle external deletion gracefully (check for missing state in Read, remove from state rather than erroring).
- Prefer MAAS-native filters over client-side iteration — the provider must scale to thousands of machines.
