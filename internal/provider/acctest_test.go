package provider

import (
	"fmt"
	"os"

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
