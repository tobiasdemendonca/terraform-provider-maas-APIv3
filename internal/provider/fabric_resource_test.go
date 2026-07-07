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
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccFabricResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-fabric-")
	nameUpdated := "updated-" + name

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMAASFabricDestroy,
		Steps: []resource.TestStep{
			// Create with name only, class_type and description omitted
			{
				Config: testAccFabricConfig(name, nil, nil),
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
			// Update: set class_type and description to real values and change the name
			{
				Config: testAccFabricConfig(nameUpdated, strPtr("10g"), strPtr("my description")),
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
				Config: testAccFabricConfig(name, strPtr(""), strPtr("")),
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
				Config: testAccFabricConfig(name, nil, nil),
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
			// Create a second fabric with a duplicate name, expect a 409 error
			// and no new resource created
			{
				Config:      testAccFabricConfig(name, nil, nil) + testAccFabricDuplicateConfig(name),
				ExpectError: regexp.MustCompile("already exists"),
			},
			// Return to the single-resource config so the test ends on a clean,
			// no-diff state
			{
				Config: testAccFabricConfig(name, nil, nil),
			},
		},
	})
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

// Verify that the fabric resource has been destroyed in MAAS.
func testAccCheckMAASFabricDestroy(s *terraform.State) error {
	client, err := testAccNewClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "maas_fabric" {
			continue
		}
		fabricID, err := strconv.Atoi(rs.Primary.ID)
		if err != nil {
			return err
		}

		apiResp, err := client.GetFabricWithResponse(context.Background(), fabricID)
		if err != nil {
			return fmt.Errorf("error getting fabric: %s", err)
		}
		if apiResp.StatusCode() != 404 {
			return fmt.Errorf("fabric with id %d still exists (API returned %s)", fabricID, apiResp.Status())
		}
	}

	return nil
}
