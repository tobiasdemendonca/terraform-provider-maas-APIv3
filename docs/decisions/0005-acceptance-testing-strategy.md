# 0005. Acceptance testing strategy

Status: draft (2026-07-07)

## Context

Acceptance tests run against a live MAAS, so resource creation is the scarce commodity. The test framework only verifies convergence: applies succeed and post-apply plans are empty. It does not fail a destroy-and-recreate masquerading as an update, does not check the server actually holds what state claims, and only exercises provider Delete once, in the post-test destroy. The old APIv2 provider used an external test package (`maas_test`) plus a `testutils` package, forced by the SDKv2 global `TestAccProvider` and the import cycle it created.

## Decision

- One resource instance per test, driven through a full lifecycle: create, updates (including rename and clear-to-null/clear-to-literal), import, error steps (409 duplicate, 422 invalid input - failed applies persist nothing), and an out-of-band-delete drift step whose plan must be a Create (the only coverage of Read's remove-on-404).
- Plan behavior is asserted, not assumed: `plancheck.ExpectResourceAction` on every apply step pins update-in-place vs replace.
- Server-side truth is verified with an out-of-band client (`testAccNewClient`, reusing the provider's `tokenManager`) independent of the provider under test: a per-resource `CheckExists` after each apply and a `CheckDestroy` built from the generic `testAccCheckDestroy` helper. CheckDestroy receives the pre-destroy state snapshot; `rs.Primary.ID` is populated from the attribute named `id`, so resources must expose one.
- Test files stay in `package provider`: acceptance tests already black-box through the Terraform protocol, and in-package access lets helpers reuse unexported auth internals. Generic infrastructure lives in `acctest_test.go`; resource-specific closures live in the resource's test file.

## Rejected alternatives

- External `maas_test` + `testutils` packages: their motivations (SDKv2 `Meta()` global, import cycle) do not exist under the plugin framework. Revisit if resources split into multiple packages.
- Relying on state checks alone: passes when the server disagrees with state and when updates silently become replaces.

## Consequences

- Each resource costs one create per test run while covering CRUD, errors, import, and drift.
- The post-test destroy plus CheckDestroy is the Delete test; breaking Delete fails the run (verified empirically).
- Out-of-band steps are order-dependent: every step that changes the resource's identity must carry the exists check so captured ids stay fresh.
