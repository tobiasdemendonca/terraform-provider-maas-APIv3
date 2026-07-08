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

func TestAccZoneResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-zone")
	nameUpdated := "updated-" + name

	var zoneID int

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMAASZoneDestroy,
		Steps: []resource.TestStep{
			// Create with name only; description omitted coerces to "".
			{
				Config: testAccZoneConfig(name, nil),
				Check:  testAccZoneCheckExists("maas_zone.test", &zoneID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_zone.test", plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_zone.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(name),
					),
					statecheck.ExpectKnownValue(
						"maas_zone.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
				},
			},
			// Update: set description and rename, in-place.
			{
				Config: testAccZoneConfig(nameUpdated, strPtr("my description")),
				Check:  testAccZoneCheckExists("maas_zone.test", &zoneID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_zone.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_zone.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(nameUpdated),
					),
					statecheck.ExpectKnownValue(
						"maas_zone.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("my description"),
					),
				},
			},
			// Import by ID
			{
				ResourceName:      "maas_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update: set description="" explicitly
			{
				Config: testAccZoneConfig(nameUpdated, strPtr("")),
				Check:  testAccZoneCheckExists("maas_zone.test", &zoneID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_zone.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_zone.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
				},
			},
			// 422 on an invalid name, no state change.
			{
				Config:      testAccZoneConfig("bad!name", nil),
				ExpectError: regexp.MustCompile(`Invalid\s+entity\s+name`),
			},
			// 409 on a duplicate name, no new resource.
			{
				Config: testAccZoneConfig(nameUpdated, nil) + testAccZoneDuplicateConfig(nameUpdated),
				// \s+ tolerates a line break between words
				ExpectError: regexp.MustCompile(`already\s+exists`),
			},
			// Out-of-band delete, expect drift detection and recreate.
			{
				PreConfig: testAccDeleteZoneByID(t, &zoneID),
				Config:    testAccZoneConfig(nameUpdated, nil),
				Check:     testAccZoneCheckExists("maas_zone.test", &zoneID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_zone.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

var testAccCheckMAASZoneDestroy = testAccCheckDestroy("maas_zone",
	func(ctx context.Context, client *maasclientv3.ClientWithResponses, id int) (bool, error) {
		apiResp, err := client.GetZoneWithResponse(ctx, id)
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

// testAccZoneCheckExists verifies the zone exists in MAAS and captures its id.
func testAccZoneCheckExists(rn string, zoneID *int) resource.TestCheckFunc {
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
		apiResp, err := client.GetZoneWithResponse(context.Background(), id)
		if err != nil {
			return fmt.Errorf("error getting zone: %w", err)
		}
		if apiResp.JSON200 == nil {
			return fmt.Errorf("zone %d not found in MAAS (API returned %s)", id, apiResp.Status())
		}

		*zoneID = id
		return nil
	}
}

// testAccDeleteZoneByID deletes the zone via the API, simulating an
// out-of-band deletion. id is set by testAccZoneCheckExists in an earlier step.
func testAccDeleteZoneByID(t *testing.T, id *int) func() {
	return func() {
		client, err := testAccNewClient()
		if err != nil {
			t.Fatal(err)
		}
		delResp, err := client.DeleteZoneWithResponse(context.Background(), *id, &maasclientv3.DeleteZoneParams{})
		if err != nil {
			t.Fatal(err)
		}
		if delResp.StatusCode() != 204 {
			t.Fatalf("deleting zone %d: API returned %s", *id, delResp.Status())
		}
	}
}

// testAccZoneConfig builds a maas_zone config. nil description omits the
// attribute (coerced to "" by MAAS); a non-nil pointer emits it, even "".
func testAccZoneConfig(name string, description *string) string {
	var descriptionAttr string
	if description != nil {
		descriptionAttr = fmt.Sprintf("description = %q\n", *description)
	}
	return fmt.Sprintf(`
resource "maas_zone" "test" {
  name        = %q
  %s
}
`, name, descriptionAttr)
}

func testAccZoneDuplicateConfig(name string) string {
	return fmt.Sprintf(`
resource "maas_zone" "dup" {
  name = %q
}
`, name)
}
