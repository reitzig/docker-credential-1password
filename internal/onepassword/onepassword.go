package onepassword

import (
	"context"
	"fmt"
	"os"

	"github.com/1password/onepassword-sdk-go"
	"github.com/docker-credential-1password/internal/app"
	"github.com/docker-credential-1password/internal/config"
	"github.com/docker-credential-1password/internal/logging"
)

type Auth struct {
	Username string
	Secret   string
}

type RegistriesAndUsernames map[string]string

type OpClient struct {
	config    config.Config
	sdkClient sdkClient
}

type sdkClient interface {
	Secrets() onepassword.SecretsAPI
}

func Client(settings config.Config) (*OpClient, error) {
	account, err := settings.AccountName()
	if err != nil {
		return nil, err
	}

	sdkClient, err := onepassword.NewClient(context.Background(),
		onepassword.WithDesktopAppIntegration(account),
		onepassword.WithIntegrationInfo(app.Name, app.Version),
	)
	if err != nil {
		return nil, err
	}

	return &OpClient{
		config:    settings,
		sdkClient: sdkClient,
	}, nil
}

func (client *OpClient) AuthFor(registry string) (Auth, error) {
	opRefs, err := client.config.RefsFor(registry)
	if err != nil {
		return Auth{}, err
	}

	logging.Debug("will retrieve secrets: %q, %q", opRefs.Username.URI(), opRefs.Secret.URI())
	responses, err := client.sdkClient.Secrets().ResolveAll(context.Background(), []string{
		opRefs.Username.URI(),
		opRefs.Secret.URI(),
	})
	if err != nil {
		return Auth{}, err
	}

	auth := Auth{}
	for ref, response := range responses.IndividualResponses {
		logging.Debug("checking response for secret reference: %q", ref)
		if response.Error != nil {
			return Auth{}, fmt.Errorf("secret retrieval failed: %v", *response.Error)
		} else if ref == opRefs.Username.URI() {
			auth.Username = response.Content.Secret
		} else if ref == opRefs.Secret.URI() {
			auth.Secret = response.Content.Secret
		} else {
			return Auth{}, fmt.Errorf("received unexpected secret for '%s'", ref)
		}
	}
	if auth.Username == "" || auth.Secret == "" {
		return Auth{}, fmt.Errorf("received incomplete response")
	}

	return auth, nil
}

func (client *OpClient) ListUsers() (RegistriesAndUsernames, error) {
	nameRefs := client.config.UsernameRefs()

	names := make(RegistriesAndUsernames, len(nameRefs))

	for reg, nameRef := range nameRefs {
		response, err := client.sdkClient.Secrets().Resolve(context.Background(), nameRef)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Could not retrieve username for '%s': %v\n", reg, err)
		} else {
			names[reg] = response
		}
	}

	return names, nil
}
