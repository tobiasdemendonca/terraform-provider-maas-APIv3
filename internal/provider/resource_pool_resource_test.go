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

func TestAccResourcePoolResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-resource-pool")
	nameUpdated := "updated-" + name

	var poolID int

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMAASResourcePoolDestroy,
		Steps: []resource.TestStep{
			// Create with name only; description omitted coerces to "".
			{
				Config: testAccResourcePoolConfig(name, nil),
				Check:  testAccResourcePoolCheckExists("maas_resource_pool.test", &poolID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_resource_pool.test", plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_resource_pool.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(name),
					),
					statecheck.ExpectKnownValue(
						"maas_resource_pool.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
				},
			},
			// Update: set description and rename, in-place.
			{
				Config: testAccResourcePoolConfig(nameUpdated, strPtr("my description")),
				Check:  testAccResourcePoolCheckExists("maas_resource_pool.test", &poolID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_resource_pool.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_resource_pool.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(nameUpdated),
					),
					statecheck.ExpectKnownValue(
						"maas_resource_pool.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("my description"),
					),
				},
			},
			// Import by ID
			{
				ResourceName:      "maas_resource_pool.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update: set description="" explicitly
			{
				Config: testAccResourcePoolConfig(nameUpdated, strPtr("")),
				Check:  testAccResourcePoolCheckExists("maas_resource_pool.test", &poolID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_resource_pool.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_resource_pool.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(""),
					),
				},
			},
			// 422 on an invalid name, no state change.
			{
				Config:      testAccResourcePoolConfig("bad!name", nil),
				ExpectError: regexp.MustCompile(`Invalid\s+entity\s+name`),
			},
			// 409 on a duplicate name, no new resource.
			{
				Config: testAccResourcePoolConfig(nameUpdated, nil) + testAccResourcePoolDuplicateConfig(nameUpdated),
				// \s+ tolerates a line break between words
				ExpectError: regexp.MustCompile(`already\s+exists`),
			},
			// Out-of-band delete, expect drift detection and recreate.
			{
				PreConfig: testAccDeleteResourcePoolByID(t, &poolID),
				Config:    testAccResourcePoolConfig(nameUpdated, nil),
				Check:     testAccResourcePoolCheckExists("maas_resource_pool.test", &poolID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_resource_pool.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

var testAccCheckMAASResourcePoolDestroy = testAccCheckDestroy("maas_resource_pool",
	func(ctx context.Context, client *maasclientv3.ClientWithResponses, id int) (bool, error) {
		apiResp, err := client.GetResourcePoolWithResponse(ctx, id)
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

// testAccResourcePoolCheckExists verifies the resource pool exists in MAAS and
// captures its id.
func testAccResourcePoolCheckExists(rn string, poolID *int) resource.TestCheckFunc {
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
		apiResp, err := client.GetResourcePoolWithResponse(context.Background(), id)
		if err != nil {
			return fmt.Errorf("error getting resource pool: %w", err)
		}
		if apiResp.JSON200 == nil {
			return fmt.Errorf("resource pool %d not found in MAAS (API returned %s)", id, apiResp.Status())
		}

		*poolID = id
		return nil
	}
}

// testAccDeleteResourcePoolByID deletes the resource pool via the API,
// simulating an out-of-band deletion. id is set by
// testAccResourcePoolCheckExists in an earlier step.
func testAccDeleteResourcePoolByID(t *testing.T, id *int) func() {
	return func() {
		client, err := testAccNewClient()
		if err != nil {
			t.Fatal(err)
		}
		delResp, err := client.DeleteResourcePoolWithResponse(context.Background(), *id, &maasclientv3.DeleteResourcePoolParams{})
		if err != nil {
			t.Fatal(err)
		}
		if delResp.StatusCode() != 204 {
			t.Fatalf("deleting resource pool %d: API returned %s", *id, delResp.Status())
		}
	}
}

// testAccResourcePoolConfig builds a maas_resource_pool config. nil description
// omits the attribute (coerced to "" by MAAS); a non-nil pointer emits it,
// even "".
func testAccResourcePoolConfig(name string, description *string) string {
	var descriptionAttr string
	if description != nil {
		descriptionAttr = fmt.Sprintf("description = %q\n", *description)
	}
	return fmt.Sprintf(`
resource "maas_resource_pool" "test" {
  name        = %q
  %s
}
`, name, descriptionAttr)
}

func testAccResourcePoolDuplicateConfig(name string) string {
	return fmt.Sprintf(`
resource "maas_resource_pool" "dup" {
  name = %q
}
`, name)
}
