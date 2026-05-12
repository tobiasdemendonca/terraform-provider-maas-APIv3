See [AGENTS.md](AGENTS.md) for the practical developer and agent guide (commands, layout, conventions).

Abstract
The previous provider has been successful in allowing customers to manage MAAS resources via APIv2 using the existing terraform-provider-maas. This spec outlines a plan for developing a new provider specifically for APIv3.
Rationale
Since the current terraform-provider-maas was  created, APIv3 has been conceived. A terraform provider is required due to:

The eventual deprecation of the APIv2 in future MAAS versions.
New authentication methods.
Deployment profiles, giving users a clearer way of managing machine life cycles.
New features such as switch provisioning.
Improved handling of existing resources such as images, which will improve resource reliability and remove excessive business logic workarounds in the provider.

Creating a new provider is not the only solution to address the above problems. An alternative is to add APIv3 resources to the current provider. This is not favored for several reasons:

Creating a new provider aligns with the Hashicorp's (the company behind Terraform) provider design principles that there should be one provider per api.
Simple implementation and UX of authentication (OAuth 1.0 vs OAuth2). This also adds regression risk as we add new logic to the existing provider.
There will be multiple Terraform resources for a single MAAS entity. This will provide confusing options to the user, and make state management fragile. The latter could be mitigated with documentation for simple resources, but it does not provide a clear and simple UX.
Allows us as developers to move towards using the Terraform provider framework over the existing older SDKv2. The comparison between the two is not in the scope of this document, but more information can be found here.
Independent release cycles allow for different release cadences, allowing us to deliver releases faster for both providers.
Clearer ownership - this will simplify development processes as the v2 provider will be handed off to SEG to continue compatibility with older MAAS versions, whilst the MAAS team can continue to develop on the new provider.
Better dependency hygiene for both providers.
Specification
The following section is a wish list of everything we'd like from the new provider. This is a mixture of developer quality of life features, and feature wish lists that will shape the provider architecture.

Framework


Platform compatibility

Terraform is the primary target, with OpenTofu compatibility considered desirable but not essential. Terraform-specific features (e.g. actions) may be used where they offer clear value.
Provider aliases to support multi-MAAS deployments should be considered during development, i.e. avoid global/package level state.

Resources and data sources

Every resource adheres to the standard Terraform resource lifecycle. Adoption patterns should be avoided in favor of imports where possible, and externally-deleted resources must be handled gracefully (see changes on this PR as an example).
Best practices should be 
Typical resource usage must be idempotent. Where a resource requires an import step to be created (e.g. a default commissioning image), a workaround must be provided to avoid multi-step deployments. This takes priority over the earlier point on resource lifecycles, as this enables reusable modules and integration with tools such as Terragrunt.
Comprehensive data sources should be made available to enable filtering of available resources. An example rationale for this is the failed commissioning of 1 machine should not prevent the deployment of all other machines.
The provider must be able to scale to large deployment sizes (i.e. thousands of deployments). This manifests itself as:
Favor using native MAAS filters for entity selection, i.e. machine id's over hostnames, to avoid having to iterate over thousands of machines.

API and version compatibility

Explicit API version negotiation between provider and MAAS instance is required for provider - MAAS compatibility.
Compatibility from MAAS version (unsure which minimum MAAS Version this should be compatible with yet), continuing forward to future MAAS versions where apiv3 is supported (*Clarification needed on responsibility boundary here, as older MAAS versions have some but not all APIv3 endpoints).
A clear deprecation policy.
The provider should facilitate Terragrunt commands, such as terragrunt stack run plan. This may require modes to disable api calls, for example for version negotiation, during provider initialization.

Testing and CI

A stable, predictable CI pipeline. 
Test execution ordering defined entirely within the repository itself.
Upgrade and migration tests as part of release CI/CD.
Nightly MAAS version matrix runs for compatibility coverage.
Acceptance tests should be idempotent - i.e. not leave trailing resources, and not leaving configuration values changed from their original. 
A complete mocked HTTP layer to enable broader unit test coverage without a live MAAS instance (this was suggested by AI and maybe is to be considered, this is likely a lot of effort and maybe not required).
Chaos testing.

Observability and debugging

Well considered structured logging throughout.
A debug mode with debugger support.
Telemetry (to consider?).

Automation and AI

The design and development of this provider should adhere to an automation first philosophy. 
Clients and boilerplate should be auto-generated where possible. This is primarily:
The go MAAS client from the OpenAPI spec
Where client generation does not work as desired, consider OpenAPI overlays where fixing the spec directly in MAAS is not possible. 
In theory, APIv3 should be backwards compatible with all MAAS versions that we have to support, so we should not need to consider multiple clients specific to a particular MAAS version.
Trivial provider resources - using OpenAPI provider spec generator for generating a provider code specification.
The repository must be structured to facilitate LLM and community contributions. Public-facing documentation should include Architectural Decision Records (ADRs), version compatibility matrices, and future milestones. These also facilitate community contributions.
Concise, well-documented style guides and process documents: AGENTS.md, CLAUDE.md, and CONTRIBUTING.md.
Experimentation with agentic, test-driven development workflows — including providing an agent with access to a live MAAS instance, the MAAS codebase, and the existing provider for maximum context.
PR and issue templates, to facilitate community contribution.
