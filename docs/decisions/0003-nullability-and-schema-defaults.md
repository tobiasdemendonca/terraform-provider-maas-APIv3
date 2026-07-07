# 0003. Nullability and schema-default patterns

Status: draft (2026-07-07)

## Context

The OpenAPI spec describes wire types, not storage. Three server layers sit behind it and each can change what a field round-trips as: pydantic request validators (e.g. `FabricRequest.description` coerces null to ""), database column constraints (the ground truth for whether null is storable), and service-layer hooks. Reading nullability off the request type (`str | None`) gives wrong schemas that produce perpetual diffs (omitted field: config null vs state "") or values that cannot be cleared (Computed without Default preserves prior state).

## Decision

For every optional attribute, ask one question: what does MAAS store for "absent"?

- A known literal ("", 0, false): `Optional + Computed + Default(literal)`. Computed is mechanically required by Default; it does not mean the server computes the value.
- A real, meaningful null: `Optional` only. Never Computed (it traps the value so it cannot be cleared to null).
- `Optional + Computed` without Default is reserved for genuinely server-populated values.

Answer the question from the MAAS source (validators, column constraints, hooks), never from the spec. The flatten function mirrors storage semantics: nil coerces to "" for non-nullable fields, nil stays null where null is meaningful, even when both response fields are `*string`.

The schema generator cannot express null defaults and emits the trap pattern (`Optional + Computed`, no Default) for them; every generated schema is hand-reviewed against the storage layer.

## Rejected alternatives

- Trusting spec/request types: wrong layer, causes both traps above.
- Uniform `Optional + Computed` for all optional fields: cannot clear to null, hides drift.

## Consequences

- Schema work requires reading MAAS source, not just the spec.
- Flatten logic legitimately differs per field of the same Go type; unit tests pin it.
- AGENTS.md carries the how-to (patterns table, traps, worked examples); `fabric_resource.go` (mixed semantics) and `tag_resource.go` (all literal) are the exemplars.
