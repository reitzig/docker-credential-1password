package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/1password/onepassword-sdk-go"
)

const appName = "docker-credential-1password"
const configFilename = "credential-1password.json"

var appVersion = "dev" // goreleaser will inject real value

type DockerAuth struct {
	Username string
	Secret   string
}

type Config struct {
	Account    string
	SecretRefs map[string]AuthSecretRefs
}

type AuthSecretRefs struct {
	Username AuthSecretRef
	Secret   AuthSecretRef
}

type AuthSecretRef struct {
	Vault string
	Item  string
	Field string
}

func (r AuthSecretRef) asURI() string {
	return fmt.Sprintf("op://%s/%s/%s", r.Vault, r.Item, r.Field)
}

// As per https://docs.docker.com/reference/cli/docker/#configuration-files
func configFile() (string, error) {
	homeDir, homeSet := os.LookupEnv("HOME")
	customDir, customSet := os.LookupEnv("DOCKER_CONFIG")

	if customSet {
		return filepath.Join(customDir, configFilename), nil
	} else if homeSet {
		return filepath.Join(homeDir, ".docker", configFilename), nil
	}

	return "", fmt.Errorf("can not locate Docker client config dir; set 'HOME' or 'DOCKER_CONFIG'")
}

func readConfig() (Config, error) {
	config := Config{}
	file, err := configFile()
	if err != nil {
		return config, err
	}

	if exists, err := fileExists(file); err != nil {
		return config, fmt.Errorf("error accessing config file '%s': %v", file, err)
	} else if exists {
		//fmt.Printf("Found config file: %s\n", file) // TODO remove or log properly
		content, err := os.ReadFile(file)
		//fmt.Printf("Read config JSON: %s\n", string(content)) // TODO remove or log properly
		if err != nil {
			return config, fmt.Errorf("error reading config file '%s': %v", file, err)
		}

		err = json.Unmarshal(content, &config)
		if err != nil {
			return config, fmt.Errorf("error parsing config file '%s': %v", file, err)
		}
	}

	return config, nil
}

func registryMatches(registryURLA string, registryURLB string) bool {
	return strings.TrimPrefix(strings.TrimSuffix(registryURLA, "/"), "https://") ==
		strings.TrimPrefix(strings.TrimSuffix(registryURLB, "/"), "https://")
}

func opRefFor(registry string) (AuthSecretRefs, error) {
	config, err := readConfig()
	if err != nil {
		return AuthSecretRefs{}, err
	}

	for reg, refs := range config.SecretRefs {
		if registryMatches(reg, registry) {
			return refs, nil
		}
	}

	// TODO: ask user for it; store to file
	return AuthSecretRefs{}, fmt.Errorf("NYI: ask user for missing refs")
}

func accountName() (string, error) {
	config, err := readConfig()
	if err != nil {
		return "", err
	}
	//fmt.Printf("found config: %+v\n", config) // TODO remove or log properly

	if config.Account == "" {
		// TODO: ask user for it; store to file
		return "", fmt.Errorf("NYI: ask user for missing account name")
	}
	//_, _ = fmt.Printf("found Account: %s\n", config.Account) // TODO remove or log properly

	return config.Account, nil
}

func opClient() (*onepassword.Client, error) {
	account, err := accountName()
	if err != nil {
		return nil, err
	}

	// TODO: support OP_SERVICE_ACCOUNT_TOKEN ?
	return onepassword.NewClient(context.Background(),
		onepassword.WithDesktopAppIntegration(account),
		onepassword.WithIntegrationInfo(appName, appVersion),
	)
}

func authFor(registry string) (DockerAuth, error) {
	opRefs, err := opRefFor(registry)
	if err != nil {
		return DockerAuth{}, err
	}

	client, err := opClient()
	if err != nil {
		return DockerAuth{}, err
	}

	//fmt.Printf("will retrieve secrets: '%s', '%s'\n", opRefs.Username.asUri(), opRefs.Secret.asUri()) // TODO remove or log properly
	responses, err := client.Secrets().ResolveAll(context.Background(), []string{
		opRefs.Username.asURI(),
		opRefs.Secret.asURI(),
	})
	if err != nil {
		return DockerAuth{}, err
	}

	dockerAuth := DockerAuth{}
	for ref, response := range responses.IndividualResponses {
		//fmt.Printf("Checking response for: '%s'\n", ref)  // TODO remove or log properly
		if response.Error != nil {
			return DockerAuth{}, fmt.Errorf("secret retrieval failed: %v", *response.Error)
		} else if ref == opRefs.Username.asURI() {
			dockerAuth.Username = response.Content.Secret
		} else if ref == opRefs.Secret.asURI() {
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

func allRegistriesAndUsernames() (map[string]string, error) {
	config, err := readConfig()
	if err != nil {
		return map[string]string{}, err
	}

	nameRefs := map[string]string{}
	for reg, refs := range config.SecretRefs {
		nameRefs[reg] = refs.Username.asURI()
	}

	client, err := opClient()
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

func fileExists(name string) (bool, error) {
	info, err := os.Stat(name)

	if err == nil {
		return !info.IsDir(), nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func readRegistryURL() (string, error) {
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
			registryURL, err := readRegistryURL()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to read registry URL from stdin: %v\n", err)
				os.Exit(1)
			}

			auth, err := authFor(registryURL)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Could not retrieve credentials for '%s': %v\n", registryURL, err)
				os.Exit(2)
			}

			data, err := json.Marshal(auth)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to encode auth as JSON: %v\n", err)
				os.Exit(3)
			}

			fmt.Println(string(data))
		case "list":
			names, err := allRegistriesAndUsernames()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Could not retrieve usernames': %v\n", err)
				os.Exit(2)
			}

			data, err := json.Marshal(names)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to encode usernames as JSON: %v\n", err)
				os.Exit(3)
			}

			fmt.Println(string(data))

		case "store", "erase":
			_, _ = fmt.Fprintf(os.Stderr, "Command '%s' not implemented; manage secrets through 1Password\n", command)
			os.Exit(4)
		default:
			_, _ = fmt.Fprintf(os.Stderr, "Unknown command '%s'; use 'get'\n", command)
			os.Exit(5)
		}
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "No command given; use 'get'\n")
		os.Exit(6)
	}
}
