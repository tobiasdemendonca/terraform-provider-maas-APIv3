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

func TestAccUserResource(t *testing.T) {
	username := acctest.RandomWithPrefix("tf-user")
	usernameUpdated := username + "-updated"
	emailUpdated := username + "-updated@example.com"
	password := "P8ssw0rd12"
	passwordUpdated := "NewP8ssw0rd99"

	var userID int

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Create with required fields only; email omitted (can be null in MAAS)
			{
				Config: testAccUserConfig(username, "First", "Last", nil, password, "1"),
				Check: resource.ComposeTestCheckFunc(
					testAccUserCheckExists("maas_user.test", &userID),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_user.test", plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_user.test",
						tfjsonpath.New("username"),
						knownvalue.StringExact(username),
					),
					statecheck.ExpectKnownValue(
						"maas_user.test",
						tfjsonpath.New("first_name"),
						knownvalue.StringExact("First"),
					),
					statecheck.ExpectKnownValue(
						"maas_user.test",
						tfjsonpath.New("last_name"),
						knownvalue.StringExact("Last"),
					),
					statecheck.ExpectKnownValue(
						"maas_user.test",
						tfjsonpath.New("email"),
						knownvalue.Null(),
					),
				},
			},
			// Update: change username, first_name, last_name, set email, bump
			// password version
			{
				Config: testAccUserConfig(usernameUpdated, "UpdatedFirst", "UpdatedLast", strPtr(emailUpdated), passwordUpdated, "2"),
				Check: resource.ComposeTestCheckFunc(
					testAccUserCheckExists("maas_user.test", &userID),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_user.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_user.test",
						tfjsonpath.New("username"),
						knownvalue.StringExact(usernameUpdated),
					),
					statecheck.ExpectKnownValue(
						"maas_user.test",
						tfjsonpath.New("first_name"),
						knownvalue.StringExact("UpdatedFirst"),
					),
					statecheck.ExpectKnownValue(
						"maas_user.test",
						tfjsonpath.New("last_name"),
						knownvalue.StringExact("UpdatedLast"),
					),
					statecheck.ExpectKnownValue(
						"maas_user.test",
						tfjsonpath.New("email"),
						knownvalue.StringExact(emailUpdated),
					),
				},
			},
			// Import by username
			{
				ResourceName:            "maas_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password_wo", "password_wo_version", "transfer_resources_to_wo"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["maas_user.test"]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					return rs.Primary.Attributes["username"], nil
				},
			},
			// Clear email back to null (can be null in MAAS)
			{
				Config: testAccUserConfig(usernameUpdated, "UpdatedFirst", "UpdatedLast", nil, passwordUpdated, "2"),
				Check: resource.ComposeTestCheckFunc(
					testAccUserCheckExists("maas_user.test", &userID),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_user.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"maas_user.test",
						tfjsonpath.New("email"),
						knownvalue.Null(),
					),
				},
			},
			// Update password version without changing other fields; password
			// is applied, no other attribute changes
			{
				Config: testAccUserConfig(usernameUpdated, "UpdatedFirst", "UpdatedLast", nil, "ThirdP8ssw0rd", "3"),
				Check: resource.ComposeTestCheckFunc(
					testAccUserCheckExists("maas_user.test", &userID),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_user.test", plancheck.ResourceActionUpdate),
					},
				},
			},
			// Attempt to create a user with a duplicate username, expect 409
			{
				Config: testAccUserConfig(usernameUpdated, "UpdatedFirst", "UpdatedLast", nil, passwordUpdated, "2") +
					testAccUserDuplicateConfig(usernameUpdated),
				ExpectError: regexp.MustCompile(`already\s+exists`),
			},
			// Attempt to update with an invalid email, expect 422
			{
				Config:      testAccUserConfig(usernameUpdated, "UpdatedFirst", "UpdatedLast", strPtr("not-an-email"), passwordUpdated, "2"),
				ExpectError: regexp.MustCompile(`valid\s+email`),
			},
			// Delete the user outside of Terraform, expect refresh to detect
			// drift and plan to recreate
			{
				PreConfig: testAccDeleteUserByID(t, &userID),
				Config:    testAccUserConfig(usernameUpdated, "UpdatedFirst", "UpdatedLast", nil, passwordUpdated, "2"),
				Check: resource.ComposeTestCheckFunc(
					testAccUserCheckExists("maas_user.test", &userID),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("maas_user.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

var testAccCheckUserDestroy = testAccCheckDestroy("maas_user",
	func(ctx context.Context, client *maasclientv3.ClientWithResponses, id int) (bool, error) {
		apiResp, err := client.GetUserWithResponse(ctx, id)
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

func testAccUserCheckExists(rn string, userID *int) resource.TestCheckFunc {
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
		apiResp, err := client.GetUserWithResponse(context.Background(), id)
		if err != nil {
			return fmt.Errorf("error getting user: %w", err)
		}
		if apiResp.JSON200 == nil {
			return fmt.Errorf("user %d not found in MAAS (API returned %s)", id, apiResp.Status())
		}

		*userID = id
		return nil
	}
}

func testAccDeleteUserByID(t *testing.T, id *int) func() {
	return func() {
		client, err := testAccNewClient()
		if err != nil {
			t.Fatal(err)
		}
		delResp, err := client.DeleteUserWithResponse(context.Background(), *id, &maasclientv3.DeleteUserParams{})
		if err != nil {
			t.Fatal(err)
		}
		if delResp.StatusCode() != 204 {
			t.Fatalf("deleting user %d: API returned %s", *id, delResp.Status())
		}
	}
}

func testAccUserConfig(username, firstName, lastName string, email *string, passwordWo, passwordWoVersion string) string {
	emailLine := ""
	if email != nil {
		emailLine = fmt.Sprintf("  email               = %q\n", *email)
	}
	return fmt.Sprintf(`
resource "maas_user" "test" {
  username            = %q
  first_name          = %q
  last_name           = %q
%s  password_wo         = %q
  password_wo_version = %q
}
`, username, firstName, lastName, emailLine, passwordWo, passwordWoVersion)
}

func testAccUserDuplicateConfig(username string) string {
	return fmt.Sprintf(`
resource "maas_user" "duplicate" {
  username            = %q
  first_name          = "Dup"
  last_name           = "User"
  password_wo         = "P8ssw0rd12"
  password_wo_version = "1"
}
`, username)
}
