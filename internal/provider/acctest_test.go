package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

// testAccNewClient returns an authenticated MAAS client for client usage in tests.
func testAccNewClient() (*maasclientv3.ClientWithResponses, error) {
	url := os.Getenv(MaasURLEnvKey)
	authClient, err := maasclientv3.NewClientWithResponses(url)
	if err != nil {
		return nil, fmt.Errorf("creating MAAS auth client: %w", err)
	}
	tm := &tokenManager{
		serverURL:  url,
		username:   os.Getenv(MaasUserEnvKey),
		password:   os.Getenv(MaasPasswordEnvKey),
		authClient: authClient,
	}
	client, err := maasclientv3.NewClientWithResponses(url,
		maasclientv3.WithRequestEditorFn(tm.requestEditor),
	)
	if err != nil {
		return nil, fmt.Errorf("creating MAAS client: %w", err)
	}
	return client, nil
}

// testAccCheckDestroy returns a CheckDestroy function verifying that every
// resource of the given type in state is gone from MAAS. exists reports
// whether the resource with the given id still exists. Assumes the resource
// has an attribute named id as its primary key.
func testAccCheckDestroy(resourceType string, exists func(ctx context.Context, client *maasclientv3.ClientWithResponses, id int) (bool, error)) func(*terraform.State) error {
	return func(s *terraform.State) error {
		client, err := testAccNewClient()
		if err != nil {
			return err
		}
		for _, rs := range s.RootModule().Resources {
			if rs.Type != resourceType {
				continue
			}
			id, err := strconv.Atoi(rs.Primary.ID)
			if err != nil {
				return fmt.Errorf("invalid %s ID in state %q: %w", resourceType, rs.Primary.ID, err)
			}
			found, err := exists(context.Background(), client, id)
			if err != nil {
				return fmt.Errorf("checking %s %d: %w", resourceType, id, err)
			}
			if found {
				return fmt.Errorf("%s with id %d still exists", resourceType, id)
			}
		}
		return nil
	}
}
