package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/1password/onepassword-sdk-go"
	"github.com/docker-credential-1password/internal/app"
	"github.com/docker-credential-1password/internal/config"
	"github.com/docker-credential-1password/internal/logging"
)

var ( // goreleaser will inject real values for these
	version = "dev"
	commit  = "HEAD"
	date    = "now"
)

type DockerAuth struct {
	Username string
	Secret   string
}

func opClient(settings config.Config) (*onepassword.Client, error) {
	account, err := settings.AccountName()
	if err != nil {
		return nil, err
	}
	logging.Debug("found account: %s", account)

	// TODO: support OP_SERVICE_ACCOUNT_TOKEN ?
	return onepassword.NewClient(context.Background(),
		onepassword.WithDesktopAppIntegration(account),
		onepassword.WithIntegrationInfo(app.Name, version),
	)
}

func authFor(settings config.Config, registry string) (DockerAuth, error) {
	opRefs, err := settings.RefsFor(registry)
	if err != nil {
		return DockerAuth{}, err
	}

	client, err := opClient(settings)
	if err != nil {
		return DockerAuth{}, err
	}

	logging.Debug("will retrieve secrets: %q, %q", opRefs.Username.URI(), opRefs.Secret.URI())
	responses, err := client.Secrets().ResolveAll(context.Background(), []string{
		opRefs.Username.URI(),
		opRefs.Secret.URI(),
	})
	if err != nil {
		return DockerAuth{}, err
	}

	dockerAuth := DockerAuth{}
	for ref, response := range responses.IndividualResponses {
		logging.Debug("checking response for secret reference: %q", ref)
		if response.Error != nil {
			return DockerAuth{}, fmt.Errorf("secret retrieval failed: %v", *response.Error)
		} else if ref == opRefs.Username.URI() {
			dockerAuth.Username = response.Content.Secret
		} else if ref == opRefs.Secret.URI() {
			dockerAuth.Secret = response.Content.Secret
		} else {
			return DockerAuth{}, fmt.Errorf("received unexpected secret for '%s'", ref)
		}
	}
	if dockerAuth.Username == "" || dockerAuth.Secret == "" {
		return DockerAuth{}, fmt.Errorf("received incomplete response")
	}

	return dockerAuth, nil
}

func allRegistriesAndUsernames(settings config.Config) (map[string]string, error) {
	nameRefs := settings.UsernameRefs()

	client, err := opClient(settings)
	if err != nil {
		return map[string]string{}, err
	}

	names := map[string]string{}

	for reg, nameRef := range nameRefs {
		response, err := client.Secrets().Resolve(context.Background(), nameRef)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Could not retrieve username for '%s': %v\n", reg, err)
		} else {
			names[reg] = response
		}
	}

	return names, nil
}

func readInput() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	registryURL, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(registryURL), nil
}

func main() {
	if len(os.Args) > 1 {
		switch command := os.Args[1]; command {
		case "get":
			registryURL, err := readInput()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to read registry URL from stdin: %v\n", err)
				os.Exit(1)
			}

			settings, err := config.Load()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
				os.Exit(2)
			}

			auth, err := authFor(settings, registryURL)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Could not retrieve credentials for '%s': %v\n", registryURL, err)
				os.Exit(3)
			}

			data, err := json.Marshal(auth)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to encode auth as JSON: %v\n", err)
				os.Exit(4)
			}

			fmt.Println(string(data))

		case "list":
			settings, err := config.Load()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
				os.Exit(2)
			}

			names, err := allRegistriesAndUsernames(settings)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Could not retrieve usernames': %v\n", err)
				os.Exit(3)
			}

			data, err := json.Marshal(names)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to encode usernames as JSON: %v\n", err)
				os.Exit(4)
			}

			fmt.Println(string(data))

		case "version":
			fmt.Printf("%s\n\nVersion: %s\nCommit: %s\nDate: %s\n", app.Name, version, commit, date)

		case "store", "erase":
			_, _ = fmt.Fprintf(os.Stderr, "Command '%s' not implemented; manage secrets through 1Password\n", command)
			os.Exit(5)
		default:
			_, _ = fmt.Fprintf(os.Stderr, "Unknown command '%s'; use 'get','list', 'version'\n", command)
			os.Exit(5)
		}
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "No command given; use 'get', 'list', 'version'\n")
		os.Exit(6)
	}
}
