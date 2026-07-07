package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccFabricResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-fabric-")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with name only — class_type and description omitted
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
			// Update — set class_type and description to real values
			{
				Config: testAccFabricConfig(name, strPtr("10g"), strPtr("my description")),
				ConfigStateChecks: []statecheck.StateCheck{
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
			// Update — set class_type="" and description="" explicitly
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
			// Update — remove class_type and description from config entirely
			// class_type: omitted → null (can be null in MAAS)
			// description: omitted → coerced to "" (not nullable in MAAS, expecting "")
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
		},
	})
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
