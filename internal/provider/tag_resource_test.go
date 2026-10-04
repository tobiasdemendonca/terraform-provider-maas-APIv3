// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTagResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccTagResourceConfig("acc-test-tag", "initial comment"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("maas_tag.test", tfjsonpath.New("name"), knownvalue.StringExact("acc-test-tag")),
					statecheck.ExpectKnownValue("maas_tag.test", tfjsonpath.New("comment"), knownvalue.StringExact("initial comment")),
				},
			},
			// Update comment
			{
				Config: testAccTagResourceConfig("acc-test-tag", "updated comment"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("maas_tag.test", tfjsonpath.New("name"), knownvalue.StringExact("acc-test-tag")),
					statecheck.ExpectKnownValue("maas_tag.test", tfjsonpath.New("comment"), knownvalue.StringExact("updated comment")),
				},
			},
			// Delete is automatic at end of TestCase
		},
	})
}

func testAccTagResourceConfig(name, comment string) string {
	return fmt.Sprintf(`
provider "maas" {}

resource "maas_tag" "test" {
  name    = %[1]q
  comment = %[2]q
}
`, name, comment)
}
