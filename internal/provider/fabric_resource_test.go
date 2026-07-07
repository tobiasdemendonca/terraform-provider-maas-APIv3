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

func TestAccFabricResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-fabric")
	nameUpdated := "updated-" + name

	// Set by testAccFabricCheckExists after each apply, used by the drift step
	// to delete the fabric outside of Terraform.
	var fabricID int

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMAASFabricDestroy,
		Steps: []resource.TestStep{
			// Create with name only, class_type and description omitted
			{
				Config: testAccFabricConfig(name, nil, nil),
				Check:  testAccFabricCheckExists("maas_fabric.test", &fabricID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_fabric.test", plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(name),
					),
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("class_type"),
						knownvalue.Null(),
					),
				},
			},
			// Update: set class_type and description to real values and change
			// the name. Ensure the update is inplace.
			{
				Config: testAccFabricConfig(nameUpdated, strPtr("10g"), strPtr("my description")),
				Check:  testAccFabricCheckExists("maas_fabric.test", &fabricID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_fabric.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(nameUpdated),
					),
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("class_type"),
						knownvalue.StringExact("10g"),
					),
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("my description"),
					),
				},
			},
			// Import by ID
			{
				ResourceName:      "maas_fabric.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update: set class_type="" and description="" explicitly
			{
				Config: testAccFabricConfig(nameUpdated, strPtr(""), strPtr("")),
				Check:  testAccFabricCheckExists("maas_fabric.test", &fabricID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_fabric.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("class_type"),
						knownvalue.StringExact(""),
					),
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
				},
			},
			// Update: remove class_type and description from config entirely
			// class_type: omitted becomes null (can be null in MAAS)
			// description: omitted is coerced to "" (not nullable in MAAS)
			{
				Config: testAccFabricConfig(nameUpdated, nil, nil),
				Check:  testAccFabricCheckExists("maas_fabric.test", &fabricID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_fabric.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("class_type"),
						knownvalue.Null(),
					),
					statecheck.ExpectKnownValue(
						"maas_fabric.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
				},
			},
			// Update to a name MAAS rejects, expect a 422 validation error and
			// no state change
			{
				Config:      testAccFabricConfig("bad!name", nil, nil),
				ExpectError: regexp.MustCompile(`Invalid\s+entity\s+name`),
			},
			// Create a second fabric with a duplicate name, expect a 409 error
			// and no new resource created
			{
				Config: testAccFabricConfig(nameUpdated, nil, nil) + testAccFabricDuplicateConfig(nameUpdated),
				// Use \s+ to tolerate a line break between words
				ExpectError: regexp.MustCompile(`already\s+exists`),
			},
			// Delete the fabric outside of Terraform, expect the refresh to
			// detect the drift and the plan to recreate it
			{
				PreConfig: testAccDeleteFabricByID(t, &fabricID),
				Config:    testAccFabricConfig(nameUpdated, nil, nil),
				Check:     testAccFabricCheckExists("maas_fabric.test", &fabricID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_fabric.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

// testAccCheckMAASFabricDestroy verifies every fabric in state is gone from MAAS.
var testAccCheckMAASFabricDestroy = testAccCheckDestroy("maas_fabric",
	func(ctx context.Context, client *maasclientv3.ClientWithResponses, id int) (bool, error) {
		apiResp, err := client.GetFabricWithResponse(ctx, id)
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

// testAccFabricCheckExists verifies the fabric in state is actually present
// in MAAS and stores its id in fabricID for later out-of-band operations.
func testAccFabricCheckExists(rn string, fabricID *int) resource.TestCheckFunc {
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
		apiResp, err := client.GetFabricWithResponse(context.Background(), id)
		if err != nil {
			return fmt.Errorf("error getting fabric: %w", err)
		}
		if apiResp.JSON200 == nil {
			return fmt.Errorf("fabric %d not found in MAAS (API returned %s)", id, apiResp.Status())
		}

		*fabricID = id
		return nil
	}
}

// testAccDeleteFabricByID deletes the fabric directly via the API, simulating
// a deletion outside of Terraform. id is a pointer because it is set by
// testAccFabricCheckExists in an earlier step, after this function is called.
func testAccDeleteFabricByID(t *testing.T, id *int) func() {
	return func() {
		client, err := testAccNewClient()
		if err != nil {
			t.Fatal(err)
		}
		delResp, err := client.DeleteFabricWithResponse(context.Background(), *id, &maasclientv3.DeleteFabricParams{})
		if err != nil {
			t.Fatal(err)
		}
		if delResp.StatusCode() != 204 {
			t.Fatalf("deleting fabric %d: API returned %s", *id, delResp.Status())
		}
	}
}

func strPtr(s string) *string {
	return &s
}

// testAccFabricConfig builds a maas_fabric resource config. A nil *string
// omits the attribute from HCL (plan sends null); a non-nil pointer emits
// the attribute with the pointed-to value, even if that value is "".
func testAccFabricConfig(name string, classType, description *string) string {
	var classTypeAttr, descriptionAttr string
	if classType != nil {
		classTypeAttr = fmt.Sprintf("class_type = %q\n", *classType)
	}
	if description != nil {
		descriptionAttr = fmt.Sprintf("description = %q\n", *description)
	}
	return fmt.Sprintf(`
resource "maas_fabric" "test" {
  name        = %q
  %s%s
}
`, name, descriptionAttr, classTypeAttr)
}

// testAccFabricDuplicateConfig builds a second fabric resource with the given
// name, used to provoke a name conflict.
func testAccFabricDuplicateConfig(name string) string {
	return fmt.Sprintf(`
resource "maas_fabric" "dup" {
  name = %q
}
`, name)
}
