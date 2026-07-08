# AGENTS.md

## Project

Terraform provider for the MAAS API v3, built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

The Go client is auto-generated from the OpenAPI spec via `oapi-codegen`.


## Essential Commands

See the `Makefile` for a full list of commands. Use them where possible rather than running commands directly. 

## Project Layout

```
api/
  generator_config.yaml          # experimental pipeline — add resources, paths, schema.ignores here
  generated/
    openapi.json                 # upstream MAAS OpenAPI spec (source of truth for both pipelines)
    openapi.converted.json       # gitignored — intermediate, produced by both generate-* targets
    provider-code-spec.json      # gitignored — intermediate, produced by tfplugingen-openapi
docs/
  decisions/                     # Architecture Decision Records (NNNN-title.md) - Reference these to understand key design decisions in this project
internal/
  client/
    maasclientv3/
      oapi-codegen.yaml          # Config for oapi-codegen to generate the Go client from the OpenAPI spec
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

## Generated files

Intermediate files (`openapi.converted.json`, `provider-code-spec.json`) are gitignored — they are fully derivable and committing them creates drift risk. Final Go artifacts (`client.gen.go`, `*_resource_gen.go`) **are** committed so that `go build` works without any generators installed and so that spec changes are visible as a reviewable `git diff`. Never write to generated files by hand - fix the spec or generator config instead.

### Adding a new resource
Ignore the make generate-resources target — it is experimental at this point in time. To add a new resource, follow these steps:
1. Run `make scaffold-resource NAME=<resource name>` — where resource name is the name of the new resource
2. Implement CRUD using the principles outlined in the [Key Principles](#key-principles) section below
3. Register in `provider.go`

### Updating `openapi.json` (spec bump)
This should be done if the upstream OpenAPI spec has changed. Ignore this for most PRs.
1. Replace `api/generated/openapi.json` with the new upstream spec
2. `make generate-client` — review `client.gen.go` diff for changed/added/removed API methods or types
3. `make generate-resources` — review **all** `*_resource_gen.go` diffs:
   - New field: decide whether to add it to `schema.ignores`
   - Removed field: the compiler will catch it — fix the implementation file
   - Type change: the compiler will catch it — fix the implementation file
4. `make build` to confirm no errors, then commit everything together

### Manual customisation
- **Never edit `*_gen.go` files** — changes will be silently overwritten on the next `make generate-resources`
- **Plan modifiers** (`RequiresReplace`, `UseStateForUnknown`): set directly on the attribute in the implementation file's inline `Schema()` method. Never use the post-processing mutation pattern (`s.Attributes["name"].(schema.StringAttribute); ...`) when the schema is written inline. See `fabric_resource.go` for the ideal pattern (don't compare in comments with it in other resources).
- **Description overrides**: add an `attributes.overrides` entry in `generator_config.yaml` under the resource's `schema:` key — these persist across spec updates
- **Spec typos or structural fixes**: add a transformation to `scripts/fix-openapi-nullable.py` rather than editing `openapi.json` directly

## Conventions

- **Framework**: always use `terraform-plugin-framework`. Imports from `terraform-plugin-sdk/v2` are banned by the linter.
- **Generated code**: `client.gen.go` and `*_resource_gen.go` are machine-generated — fix issues upstream (spec or `generator_config.yaml`), not in the generated files.
- **OpenAPI spec fixes**: if `api/generated/openapi.json` has issues that block codegen, add a transformation to `scripts/fix-openapi-nullable.py` rather than editing the spec directly.
- **ADRs/decisions**: document significant architectural decisions in `docs/decisions/` using the `NNNN-title.md` naming convention. See `docs/decisions/0001-record-architecture-decisions.md` for when to write one.
- **No global state**: the provider must support aliases for multi-MAAS deployments — avoid package-level variables.
- **id attribute**: every resource exposes an `id` attribute — the acceptance-test harness populates `rs.Primary.ID` from it and the destroy checks depend on it.
- **Shared helpers**: live in `internal/provider/utils.go`, in-package and unexported. Promote to a real package only when a second consumer package exists.
- **Linter disagreements**: fix once in `.golangci.yml` (e.g. `unparam` is excluded for `_test.go` files) — never scatter `//nolint` suppressions that every copier must repeat.

- Prefer MAAS-native filters over client-side iteration - the provider must scale to thousands of machines.
- Avoid comments almost always. Only add comments when the code is non-obvious or the reasoning is not captured in the ADRs.
- In public facing descriptions, avoid deviating from the MAAS API terminology unless there is a compelling reason to do so.
- **Comment style**: plain ASCII only (no arrows, no special characters). One-liner comments like `// Create with name only` — never number steps. Describe behavior, not who does it (`coerced to ""`, not `server coerces to ""`). Use MAAS terminology (`can be null in MAAS`, `not nullable in MAAS`) — not database jargon (`nullable column`, `NOT NULL column`).

## CRUD implementation

See `docs/decisions/0004-api-error-surfacing-and-404-semantics.md` for the reasoning; `fabric_resource.go` is the exemplar (don't compare in comments with it in other resources).

- Create and Update read request data from the plan, not the config — plan modifiers may have set values.
- Create surfaces already-exists errors with import guidance: practitioners expect to be told to import rather than have two configurations silently manage one resource.
- Read refreshes every attribute (surfaces drift, reduces import logic). Preserve the prior state value when the refreshed value is semantically equal (e.g. JSON property order or whitespace differences) to avoid extraneous diffs.
- Update returns only successfully modified data in the state response.
- 404 handling: Read removes state (external deletion is a normal event), Update adds an explicit error (never remove state there — the framework converts a null post-Update state into a misleading provider-bug message), Delete treats it as success.
- Use `UseStateForUnknown` on Computed attributes that are stable across updates, to avoid known-after-apply noise in plans.
- Every unexpected-status fallback goes through `apiError` (`utils.go`) so diagnostics carry the server's own message. Special-case a status only when the provider can add guidance the server cannot know (e.g. 409: suggest importing).

### Nullability, Optional/Computed/Default, and the MAAS type system

#### The single decision point

For every optional attribute, ask one question: **what does MAAS store for "absent"?**

- **A known non-null literal** (`""`, `0`, `false`) → `Optional + Computed + Default(literal)`.
- **`NULL`** → `Optional` only. No `Default`, no `Computed`.

Do not read the answer off the API request type (`str` vs `str | None`) — that's the wrong layer. The OpenAPI/Go request type tells you what the wire *accepts*, not what the server *stores*. Read it off the **DB column nullability + the response shape**.

#### The three MAAS layers behind the spec

The OpenAPI spec describes wire *types*, not runtime *behavior*. Three server layers sit behind it, and each can change what you actually read back:

1. **Pydantic request validators** (`maas/src/maasapiserver/v3/api/public/models/requests/<resource>.py`): `@field_validator` can normalize input — e.g. `FabricRequest.description` coerces `null → ""`. Invisible in the spec.
2. **Database column constraints** (`maas/src/maasserver/models/<resource>.py` or `maas/src/maasservicelayer/db/tables.py`): `null=False` means the value can never be stored as null, regardless of what the spec says. This is the ground truth for what round-trips as null.
3. **Service-layer hooks** (`maas/src/maasservicelayer/services/<resource>.py`): `pre_*`/`post_*` hooks can mutate, cascade, or reject.

When a field's null-vs-empty behavior matters (perpetual diffs, drift, can't-clear), verify against all three layers, not just the spec.

#### The four schema attribute patterns

| Pattern | Schema | When to use |
|---|---|---|
| **Required** | `Required: true` | User must set; no null. |
| **Optional-only** | `Optional: true` | Absent is a real, storable `NULL` (DB `null=True`). Null round-trips; clear-to-null works. |
| **Optional + Computed + Default** | `Optional: true, Computed: true, Default: stringdefault.StaticString("")` | Absent normalizes to a known literal (DB `NOT NULL`, `""` canonical). `Computed` is *forced* by `Default` — the framework rejects a `Default` without `Computed`. Clear-to-literal works because the Default fires on null config during update too. |
| **Optional + Computed (no Default)** | `Optional: true, Computed: true` | Server genuinely populates a value the user didn't set (server-side default, hardware discovery, server-managed list members). **Has the clear-to-null trap** — only use when the server really computes. |

**Why `Computed` appears in pattern 3 vs 4 — different reasons:**
- Pattern 3: `Computed` is a *mechanical requirement* of `stringdefault.StaticString("")`. It does **not** mean "server computes a value" — it's the price of admission for a schema-level default.
- Pattern 4: `Computed` genuinely means "the provider/server may set this." No `Default` because the server's contributed value isn't a static literal.

#### The two traps to avoid

**Trap A — perpetual diff on literal-absent fields without a Default.**
Omit → config null → send omitted → API stores `""` → Read returns `""` → state `""` → next plan: config null vs state `""` → diff forever. The `Default` fixes it by making null config → `""` in the plan.

**Trap B — can't clear a value when `Computed` is present without a Default.**
User sets `field = "x"`, later removes it → config null, prior state `"x"` → framework preserves prior state for Computed attributes → plan `"x"` → re-sends `"x"` → never clears. The `Default` unlocks clearing-to-literal. For clearing-to-null (pattern 2), you must use `Optional`-only — `Computed` would trap the value.

#### The generator's limitation

`make generate-resources` cannot express null defaults — the framework's `Default` plan modifiers only supply non-null typed values, so there is no `stringdefault.StaticNull()`. For any optional field whose spec default is `null`, the generator falls back to `Optional + Computed` (no Default) — **the trap pattern**. This is why `class_type` in `resource_fabric/fabric_resource_gen.go` is wrong (it should be `Optional`-only). Always hand-review generated schemas against the DB column + response shape.

### Marshal: `types.String` → `*string` (Create/Update request bodies)

Use the `optionalString` helper (see `utils.go`):
```go
func optionalString(s types.String) *string {
    if s.IsNull() || s.IsUnknown() {
        return nil
    }
    v := s.ValueString()
    return &v
}
```
When a `Default` has fired (pattern 3), the plan value is the literal `""` (known, non-null), so this returns `&""`. When the field is genuinely null (pattern 2, no Default), it returns `nil`.

**Wire behavior of `nil` depends on the Go struct's JSON tags:**
- `*string` + `omitempty` (e.g. `TagRequest`): nil → key **omitted** from JSON → API applies its own default. Cannot emit literal `null`.
- `*string` without `omitempty` (e.g. `FabricRequest`): nil → `"field": null` (explicit). Works when the API accepts null (`str | None`).

Both are fine for their respective APIs. The gotcha: **PATCH (partial-update) endpoints**, where omitting a key means "leave unchanged" and clearing requires explicit `null` — an `omitempty` struct couldn't clear. Check the method + struct tags when adapting.

### Unmarshal: response → `types.String` (Read/flatten)

The flatten function's input is the **response** struct, so its code must be written against the response's Go field types and the server's storage semantics — never the request's. State is a mirror of what the server *holds*, and the response is the server's report of that.

Same response Go type (`*string`), different flatten — driven by DB semantics:
- **DB `NOT NULL`, response loosely typed `*string` but practically `""`** (fabric `description`): `nil → types.StringValue("")` (defensive coercion; null is loose typing over a `""`-canonical field).
- **DB `null=True`, response genuinely nullable** (fabric `class_type`): `nil → types.StringNull()` (null is meaningful state).

For response fields that are plain `string` (non-pointer), `types.StringValue(value)` is correct — no nil possible.

### Worked examples

See `fabric_resource.go` (mixed: `description` `NOT NULL` → `Default("")`; `class_type` `null=True` → `Optional`-only) (don't compare in comments with it in other resources). Both files are annotated with the reasoning.

## Acceptance testing

See `docs/decisions/0005-acceptance-testing-strategy.md` for the reasoning; `fabric_resource_test.go` is the exemplar (don't compare in comments with it in other resources).

- One resource instance per test, driven through the full lifecycle: create, updates, import, error steps (409 duplicate, 422 invalid input — failed applies persist nothing), and an out-of-band-delete drift step. Tests must be idempotent and leave no trailing resources.
- Assert plan behavior with `plancheck.ExpectResourceAction` on every apply step — the framework alone does not fail a destroy-and-recreate masquerading as an update.
- Verify against MAAS, not just state: a per-resource `CheckExists` after each apply and a `CheckDestroy` built from the generic `testAccCheckDestroy` helper (`acctest_test.go`), both using the out-of-band client `testAccNewClient`.
- Cover omitted vs empty vs set for every optional attribute — these are the regressions the nullability patterns exist to prevent.
- Unit-test flatten/marshal helpers with table tests; nil-handling must not need a live server to verify.
- Skip tests that need heavy fixtures (e.g. delete blocked by subnets), and document why.
- `ExpectError` regexes use `\s+` between words — Terraform wraps diagnostics at about 78 columns.
- One behavior per config change per step, or a failure will not say which behavior broke.
- Prefer state-derived ids (`rs.Primary.ID`) over captured variables. Captured ids are only for `PreConfig`, which has no state access; every step that changes the resource's identity must carry the exists check so captured ids stay fresh.
- Verify framework-behavior claims empirically (e.g. break Delete and watch CheckDestroy fail) rather than by assertion.
