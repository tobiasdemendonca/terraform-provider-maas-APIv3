# 0004. API error surfacing and 404 semantics in CRUD

Status: draft (2026-07-07)

## Context

MAAS error responses share one body shape (`message` plus `details[{field, message}]`) carrying actionable text ("The Fabric 5 cannot be deleted as it still has subnets", "Invalid entity name"). Reporting only the HTTP status forces users into server logs. Separately, each CRUD operation can receive a 404 whose correct handling differs, and the plugin framework turns a null state after Update into a misleading "Missing Resource State After Update ... always an issue with the provider" diagnostic.

## Decision

Error surfacing:

- Every unexpected-status fallback goes through the shared `apiError(status, body)` helper (`utils.go`), which parses the common shape from the raw body bytes and appends message and per-field details to the status line, falling back to status alone for non-JSON bodies.
- A status is special-cased only when the provider can add guidance the server cannot know (e.g. Create 409: "use a different name or import the existing fabric").

404 semantics:

- Read: `RemoveResource`. External deletion is a normal event; the next plan recreates.
- Update: explicit `AddError` stating the resource was deleted outside Terraform. Never remove state here.
- Delete: success. The desired state is absence; 404 means already absent.

## Rejected alternatives

- Using the generated per-status typed fields (`JSON400`, `JSON422`, ...): one helper per status per resource; raw-body parsing is one helper total for an API with a uniform error shape.
- `RemoveResource` in Update: the framework emits the misleading provider-bug diagnostic instead of telling the user what happened.

## Consequences

- Diagnostics carry the server's own remediation text at no per-resource cost.
- The 404 matrix is uniform across resources and directly testable (drift step exercises Read; CheckDestroy tolerates Delete-after-gone).
