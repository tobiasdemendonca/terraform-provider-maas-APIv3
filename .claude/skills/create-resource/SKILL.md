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
   - Follow Terraform best practices for resource implementation:
   - `Read`:
    - Ignore returning errors that signify the resource is no longer existent, call the response state RemoveResource() method, and return early. The next Terraform plan will recreate the resource.
    - Refresh all possible values. This will ensure Terraform shows configuration drift and reduces import logic.
    - Preserve the prior state value if the updated value is semantically equal. For example, JSON strings that have inconsequential object property reordering or whitespace differences. This prevents Terraform from showing extraneous drift in plans.
   - `Delete`: 
    - If the API returns 404, consider it a success (the resource is already gone). Skip calling the response state RemoveResource() method. The framework automatically handles this logic with the response state if there are no error diagnostics.
   - `Update`: 
    - Get request data from the Terraform plan data over configuration data as the schema or resource may include plan modification logic which sets plan values. 
    - Only successfully modified parts of the resource should be return updated data in the state response.
    - Use the resource.UseStateForUnknown() attribute plan modifier for Computed attributes that are known to not change during resource updates. This will enhance the Terraform plan to not show => (known after apply) differences.
  - `Create`: 
   - Get request data from the Terraform plan data over configuration data as the schema or resource may include plan modification logic which sets plan values.
   - Return errors that signify there is an existing resource. Terraform practitioners expect to be notified if an existing resource needs to be imported into Terraform rather than created. This prevents situations where multiple Terraform configurations unexpectedly manage the same underlying resource.

8. **Verify** — Run `make lint fmt`, then `make build` to verify it compiles.

9. **Example** — Add 1 or more examples of the resource in `.devenv/main.tf` that tests its functionality for the user's own QA. Inform them. 

10. **Remind** — let the user know they should write acceptance tests. Tests must be idempotent and leave no trailing resources.
