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
	"sort"
	"strings"

	"github.com/1password/onepassword-sdk-go"
)

const appName = "docker-credential-1password"
const configFilename = "credential-1password.json"

var ( // goreleaser will inject real values for these
	version = "dev"
	commit  = "HEAD"
	date    = "now"
)

var debugEnvVar = strings.ToUpper(strings.ReplaceAll(appName, "-", "_")) + "_DEBUG"

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

func debugEnabled() bool {
	return os.Getenv(debugEnvVar) != ""
}

func debugLog(format string, args ...any) {
	if debugEnabled() {
		_, _ = fmt.Fprintf(os.Stderr, "debug: "+format+"\n", args...)
	}
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
		debugLog("found config file: %s", file)
		content, err := os.ReadFile(file)
		if err != nil {
			return config, fmt.Errorf("error reading config file '%s': %v", file, err)
		}

		err = json.Unmarshal(content, &config)
		if err != nil {
			return config, fmt.Errorf("error parsing config file '%s': %v", file, err)
		}

		if debugEnabled() {
			registries := make([]string, 0, len(config.SecretRefs))
			for registry := range config.SecretRefs {
				registries = append(registries, registry)
			}
			sort.Strings(registries)
			debugLog("found registries in config: %q", registries)
		}
	}

	return config, nil
}

func registryMatches(registryURLA string, registryURLB string) bool {
	return strings.TrimPrefix(strings.TrimSuffix(registryURLA, "/"), "https://") ==
		strings.TrimPrefix(strings.TrimSuffix(registryURLB, "/"), "https://")
}

func (config Config) opRefFor(registry string) (AuthSecretRefs, error) {
	for reg, refs := range config.SecretRefs {
		if registryMatches(reg, registry) {
			return refs, nil
		}
	}

	// TODO: ask user for it; store to file
	return AuthSecretRefs{}, fmt.Errorf("NYI: ask user for missing refs")
}

func (config Config) accountName() (string, error) {
	if config.Account == "" {
		// TODO: ask user for it; store to file
		return "", fmt.Errorf("NYI: ask user for missing account name")
	}
	debugLog("found account: %s", config.Account)

	return config.Account, nil
}

func opClient(config Config) (*onepassword.Client, error) {
	account, err := config.accountName()
	if err != nil {
		return nil, err
	}

	// TODO: support OP_SERVICE_ACCOUNT_TOKEN ?
	return onepassword.NewClient(context.Background(),
		onepassword.WithDesktopAppIntegration(account),
		onepassword.WithIntegrationInfo(appName, version),
	)
}

func authFor(config Config, registry string) (DockerAuth, error) {
	opRefs, err := config.opRefFor(registry)
	if err != nil {
		return DockerAuth{}, err
	}

	client, err := opClient(config)
	if err != nil {
		return DockerAuth{}, err
	}

	debugLog("will retrieve secrets: %q, %q", opRefs.Username.asURI(), opRefs.Secret.asURI())
	responses, err := client.Secrets().ResolveAll(context.Background(), []string{
		opRefs.Username.asURI(),
		opRefs.Secret.asURI(),
	})
	if err != nil {
		return DockerAuth{}, err
	}

	dockerAuth := DockerAuth{}
	for ref, response := range responses.IndividualResponses {
		debugLog("checking response for secret reference: %q", ref)
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

func allRegistriesAndUsernames(config Config) (map[string]string, error) {
	nameRefs := map[string]string{}
	for reg, refs := range config.SecretRefs {
		nameRefs[reg] = refs.Username.asURI()
	}

	client, err := opClient(config)
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

			config, err := readConfig()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
				os.Exit(2)
			}

			auth, err := authFor(config, registryURL)
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
			config, err := readConfig()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
				os.Exit(2)
			}

			names, err := allRegistriesAndUsernames(config)
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
			fmt.Printf("%s\n\nVersion: %s\nCommit: %s\nDate: %s\n", appName, version, commit, date)

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
