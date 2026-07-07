---
name: create-resource
description: Add a new Terraform resource to the provider.
---

You will be assisting to implement a new Terraform resource for the MAAS provider. Your ultimate objective is to write the simplest implementation of a new resource possible that matches every single criterion in the users design plan.

Follow these steps in order:
1. **Get the resource name** — ask the user for the resource name in snake_case (e.g. `network_interface`, `subnet`), and a short description of the design concept for the resource. 

2. **Regenerate the client** — run `make generate-client`. This converts the OpenAPI spec and regenerates `internal/client/maasclientv3/client.gen.go` which might have been updated. Fix any errors in `scripts/fix-openapi-nullable.py` rather than editing generated files.

3. Explore the `api/generated/openapi.json` and `internal/client/maasclientv3/client.gen.go` files to understand the available API endpoints and data structures. 

4. Execute the grill-me skill found in `.claude/skills/grill-me/` to stress-test and form a clear resource design picture. Use the existing `tag` entry as an example of a clean pattern.
   
4. Write a plan and show me the plan before asking if it needs to be revised before executing the next steps. The plan should include:
   - The resource name and a brief description of its purpose.
   - The client methods it will use for create/read/update/delete operations.
   - A high-level outline of the resource schema (the attributes it will have).
   - Any special considerations or edge cases that need to be handled in the implementation.

5. **Scaffold the resource** — run `make scaffold-resource NAME=<name>` (using the snake_case name). This creates `internal/provider/<name>_resource.go`.

6. **Register the resource** — add `New<Name>Resource` to the slice in `Resources()` in `internal/provider/provider.go`, following the existing `NewTagResource` pattern.

7. **Implement the resource** — fill in the scaffolded file:
   - Use types from the generated client (`internal/client/maasclientv3/client.gen.go`) for API calls
   - Use `terraform-plugin-framework` — never import `terraform-plugin-sdk/v2`
   - Map all create/read/update/delete operations through the generated client
   - Follow the **CRUD implementation** and **Nullability** sections in `AGENTS.md` (rationale in `docs/decisions/0003` and `0004`). `internal/provider/fabric_resource.go` is the exemplar (don't compare in comments with it in other resources).

8. **Verify** — Run `make lint fmt`, then `make build` to verify it compiles.

9. **Example** — Add 1 or more examples of the resource in `.devenv/main.tf` that tests its functionality for the user's own QA. Inform them. 

10. **Test** — Write acceptance tests following the **Acceptance testing** section in `AGENTS.md` (rationale in `docs/decisions/0005`), with `internal/provider/fabric_resource_test.go` as the exemplar (don't compare in comments with it in other resources).
