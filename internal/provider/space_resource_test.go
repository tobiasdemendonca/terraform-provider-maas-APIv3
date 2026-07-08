package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

func TestAccSpaceResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-space")
	nameUpdated := "updated-" + name

	// Set by testAccSpaceCheckExists after each apply, used by the drift step
	// to delete the space outside of Terraform.
	var spaceID int

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMAASSpaceDestroy,
		Steps: []resource.TestStep{
			// Create with name only, description omitted
			{
				Config: testAccSpaceConfig(name, nil),
				Check:  testAccSpaceCheckExists("maas_space.test", &spaceID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_space.test", plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_space.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(name),
					),
					statecheck.ExpectKnownValue(
						"maas_space.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
				},
			},
			// Update: set description to a real value and change the name.
			// Ensure the update is inplace.
			{
				Config: testAccSpaceConfig(nameUpdated, strPtr("my description")),
				Check:  testAccSpaceCheckExists("maas_space.test", &spaceID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_space.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_space.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(nameUpdated),
					),
					statecheck.ExpectKnownValue(
						"maas_space.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("my description"),
					),
				},
			},
			// Import by ID
			{
				ResourceName:      "maas_space.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update: set description="" explicitly
			{
				Config: testAccSpaceConfig(nameUpdated, strPtr("")),
				Check:  testAccSpaceCheckExists("maas_space.test", &spaceID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_space.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_space.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
				},
			},
			// Update: remove description from config entirely
			// description: omitted is coerced to "" (not nullable in MAAS)
			{
				Config: testAccSpaceConfig(nameUpdated, nil),
				Check:  testAccSpaceCheckExists("maas_space.test", &spaceID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_space.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
				},
			},
			// Update to a name MAAS rejects, expect a 422 validation error and
			// no state change
			{
				Config:      testAccSpaceConfig("bad!name", nil),
				ExpectError: regexp.MustCompile(`Invalid\s+entity\s+name`),
			},
			// Create a second space with a duplicate name, expect a 409 error
			// and no new resource created
			{
				Config: testAccSpaceConfig(nameUpdated, nil) + testAccSpaceDuplicateConfig(nameUpdated),
				// Use \s+ to tolerate a line break between words
				ExpectError: regexp.MustCompile(`already\s+exists`),
			},
			// Delete the space outside of Terraform, expect the refresh to
			// detect the drift and the plan to recreate it
			{
				PreConfig: testAccDeleteSpaceByID(t, &spaceID),
				Config:    testAccSpaceConfig(nameUpdated, nil),
				Check:     testAccSpaceCheckExists("maas_space.test", &spaceID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_space.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

// testAccCheckMAASSpaceDestroy verifies every space in state is gone from MAAS.
var testAccCheckMAASSpaceDestroy = testAccCheckDestroy("maas_space",
	func(ctx context.Context, client *maasclientv3.ClientWithResponses, id int) (bool, error) {
		apiResp, err := client.GetSpaceWithResponse(ctx, id)
		if err != nil {
			return false, err
		}
		switch {
		case apiResp.StatusCode() == 404:
			return false, nil
		case apiResp.JSON200 != nil:
			return true, nil
		default:
			return false, fmt.Errorf("API returned %s", apiResp.Status())
		}
	})

// testAccSpaceCheckExists verifies the space in state is actually present
// in MAAS and stores its id in spaceID for later out-of-band operations.
func testAccSpaceCheckExists(rn string, spaceID *int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("resource id not set")
		}
		id, err := strconv.Atoi(rs.Primary.ID)
		if err != nil {
			return err
		}

		client, err := testAccNewClient()
		if err != nil {
			return err
		}
		apiResp, err := client.GetSpaceWithResponse(context.Background(), id)
		if err != nil {
			return fmt.Errorf("error getting space: %w", err)
		}
		if apiResp.JSON200 == nil {
			return fmt.Errorf("space %d not found in MAAS (API returned %s)", id, apiResp.Status())
		}

		*spaceID = id
		return nil
	}
}

// testAccDeleteSpaceByID deletes the space directly via the API, simulating
// a deletion outside of Terraform. id is a pointer because it is set by
// testAccSpaceCheckExists in an earlier step, after this function is called.
func testAccDeleteSpaceByID(t *testing.T, id *int) func() {
	return func() {
		client, err := testAccNewClient()
		if err != nil {
			t.Fatal(err)
		}
		delResp, err := client.DeleteSpaceWithResponse(context.Background(), *id, &maasclientv3.DeleteSpaceParams{})
		if err != nil {
			t.Fatal(err)
		}
		if delResp.StatusCode() != 204 {
			t.Fatalf("deleting space %d: API returned %s", *id, delResp.Status())
		}
	}
}

// testAccSpaceConfig builds a maas_space resource config. A nil *string
// omits the attribute from HCL (plan sends null); a non-nil pointer emits
// the attribute with the pointed-to value, even if that value is "".
func testAccSpaceConfig(name string, description *string) string {
	var descriptionAttr string
	if description != nil {
		descriptionAttr = fmt.Sprintf("description = %q\n", *description)
	}
	return fmt.Sprintf(`
resource "maas_space" "test" {
  name = %q
  %s
}
`, name, descriptionAttr)
}

// testAccSpaceDuplicateConfig builds a second space resource with the given
// name, used to provoke a name conflict.
func testAccSpaceDuplicateConfig(name string) string {
	return fmt.Sprintf(`
resource "maas_space" "dup" {
  name = %q
}
`, name)
}
