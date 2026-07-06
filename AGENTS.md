# AGENTS.md

## Project

Terraform provider for the MAAS API v3, built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

The Go client is auto-generated from the OpenAPI spec via `oapi-codegen`.



Module: `terraform-provider-maas-apiv3`

## Essential Commands

```sh
make build              # compile
make install            # build and install binary to $GOPATH/bin
make generate           # run go generate (docs, etc.)
make generate-client    # regenerate internal/client/.../client.gen.go from openapi.json
make generate-resources # regenerate internal/provider/resource_*/[name]_resource_gen.go from openapi.json + generator_config.yaml
make fmt                # gofmt
make lint               # golangci-lint
make test               # unit tests
make testacc            # acceptance tests (requires live MAAS + TF_ACC=1)
make create-dev-overrides  # write dev.tfrc for local provider testing
```

## Project Layout

```
api/
  generator_config.yaml          # pipeline B config — add resources, paths, schema.ignores here
  generated/
    openapi.json                 # upstream MAAS OpenAPI spec (source of truth for both pipelines)
    openapi.converted.json       # gitignored — intermediate, produced by both generate-* targets
    provider-code-spec.json      # gitignored — intermediate, produced by tfplugingen-openapi
docs/
  ADRs/                          # Architecture Decision Records (NNNN-title.md)
internal/
  client/
    maasclientv3/
      oapi-codegen.yaml          # pipeline A codegen config
      client.gen.go              # generated — do not edit by hand
  provider/
    resource_<name>/
      <name>_resource_gen.go    # generated schema + model — do not edit by hand
    <name>_resource.go          # hand-written CRUD implementation
scripts/
  fix-openapi-nullable.py        # converts 3.1 nullable syntax to 3.0 for oapi-codegen
tools/
  tools.go                       # go:generate directives for tooling
GNUmakefile
```

## Code Generation

Two independent pipelines both source from `api/generated/openapi.json`.

**Pipeline A — API client** (`make generate-client`)
```
openapi.json → fix-openapi-nullable.py → openapi.converted.json → oapi-codegen → client.gen.go
```

**Pipeline B — Provider schemas** (`make generate-resources`)
```
openapi.json → fix-openapi-nullable.py → openapi.converted.json
    → tfplugingen-openapi (uses generator_config.yaml) → provider-code-spec.json
    → tfplugingen-framework → internal/provider/resource_<name>/<name>_resource_gen.go
```

Intermediate files (`openapi.converted.json`, `provider-code-spec.json`) are gitignored — they are fully derivable and committing them creates drift risk. Final Go artifacts (`client.gen.go`, `*_resource_gen.go`) **are** committed so that `go build` works without any generators installed and so that spec changes are visible as a reviewable `git diff`.

### Adding a new resource
Ignore the make generate-resources target — it is experimental at this point in time. To add a new resource, follow these steps:
1. Run `make scaffold-resource NAME=<resource name>` — where resource name is the name of the new resource — Implement CRUD using the generated model struct
2. Register in `provider.go`

### Updating `openapi.json` (spec bump)
This should be done if the upstream OpenAPI spec has changed. Ignore this for most PRs.
1. Replace `api/generated/openapi.json` with the new upstream spec
2. `make generate-client` — review `client.gen.go` diff for changed/added/removed API methods or types
3. `make generate-resources` — review **all** `*_resource_gen.go` diffs:
   - New field: decide whether to add it to `schema.ignores` or expose it (and update `flattenTag`/CRUD)
   - Removed field: the compiler will catch it — fix the implementation file
   - Type change: the compiler will catch it — fix the implementation file
4. `go build ./...` to confirm no errors, then commit everything together

### Manual customisation
- **Never edit `*_gen.go` files** — changes will be silently overwritten on the next `make generate-resources`
- **Plan modifiers** (`RequiresReplace`, `UseStateForUnknown`): add a post-processing patch block in the implementation file's `Schema()` method — see `tag_resource.go` for the pattern
- **Description overrides**: add an `attributes.overrides` entry in `generator_config.yaml` under the resource's `schema:` key — these persist across spec updates
- **Spec typos or structural fixes**: add a transformation to `scripts/fix-openapi-nullable.py` rather than editing `openapi.json` directly

## Conventions

- **Framework**: always use `terraform-plugin-framework`. Imports from `terraform-plugin-sdk/v2` are banned by the linter.
- **Generated code**: `client.gen.go` and `*_resource_gen.go` are machine-generated — fix issues upstream (spec or `generator_config.yaml`), not in the generated files.
- **OpenAPI spec fixes**: if `api/generated/openapi.json` has issues that block codegen, add a transformation to `scripts/fix-openapi-nullable.py` rather than editing the spec directly.
- **ADRs**: document significant architectural decisions in `docs/ADRs/` using the `NNNN-title.md` naming convention.
- **No global state**: the provider must support aliases for multi-MAAS deployments — avoid package-level variables.

## Key Constraints

- Acceptance tests must be idempotent (no trailing resources, no changed config values).
- Resources must handle external deletion gracefully (check for missing state in Read, remove from state rather than erroring).
- Prefer MAAS-native filters over client-side iteration — the provider must scale to thousands of machines.
