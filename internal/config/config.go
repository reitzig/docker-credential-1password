package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker-credential-1password/internal/app"
	"github.com/docker-credential-1password/internal/logging"
)

var filename = strings.TrimPrefix(app.Name, "docker-") + ".json"

var loadedConfig string

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

func (r AuthSecretRef) URI() string {
	return fmt.Sprintf("op://%s/%s/%s", r.Vault, r.Item, r.Field)
}

func Load() (Config, error) {
	config := Config{}
	file, err := filePath()
	if err != nil {
		return config, err
	}

	if exists, err := fileExists(file); err != nil {
		return config, fmt.Errorf("error accessing config file '%s': %v", file, err)
	} else if exists {
		logging.Debug("found config file: %s", file)

		content, err := os.ReadFile(file)
		if err != nil {
			return config, fmt.Errorf("error reading config file '%s': %v", file, err)
		}

		err = json.Unmarshal(content, &config)
		if err != nil {
			return config, fmt.Errorf("error parsing config file '%s': %v", file, err)
		}

		if logging.DebugEnabled {
			if account := config.Account; account != "" {
				logging.Debug("found account ID in config: '%s'", account)
			} else {
				logging.Debug("no account ID in config")
			}
			registries := config.RegistryNames()
			logging.Debug("found registries in config: %q", registries)
		}
	} else {
		logging.Debug("no config file found; proceeding with empty values")
	}

	loadedConfig = file // even if it doesn't exist, this _is_ where we're looking!
	return config, nil
}

func (config Config) AccountName() (string, error) {
	if config.Account == "" {
		// TODO: implement fallback to (interactively) determine the account name
		return "", fmt.Errorf("no account ID found in config '%s'", loadedConfig)
	}

	return config.Account, nil
}

func (config Config) RegistryNames() []string {
	registries := make([]string, 0, len(config.SecretRefs))
	for registry := range config.SecretRefs {
		registries = append(registries, registry)
	}
	sort.Strings(registries)
	return registries
}

func (config Config) UsernameRefs() map[string]string {
	refs := make(map[string]string, len(config.SecretRefs))
	for registry, authRefs := range config.SecretRefs {
		refs[registry] = authRefs.Username.URI()
	}
	return refs
}

func (config Config) RefsFor(registry string) (AuthSecretRefs, error) {
	for configuredRegistry, refs := range config.SecretRefs {
		if registryMatches(configuredRegistry, registry) {
			return refs, nil
		}
	}

	// TODO: implement fallback to (interactively) determine references
	return AuthSecretRefs{}, fmt.Errorf("registry '%s' not found in config '%s'", registry, loadedConfig)
}

// As per https://docs.docker.com/reference/cli/docker/#configuration-files
func filePath() (string, error) {
	homeDir, homeSet := os.LookupEnv("HOME")
	customDir, customSet := os.LookupEnv("DOCKER_CONFIG")

	if customSet && customDir != "" {
		return filepath.Join(customDir, filename), nil
	} else if homeSet {
		return filepath.Join(homeDir, ".docker", filename), nil
	}

	return "", fmt.Errorf("can not locate Docker client config dir; set 'HOME' or 'DOCKER_CONFIG'")
}

func fileExists(name string) (bool, error) {
	info, err := os.Stat(name)
	if err == nil {
		return !info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func registryMatches(registryURLA string, registryURLB string) bool {
	return strings.TrimPrefix(strings.TrimSuffix(registryURLA, "/"), "https://") ==
		strings.TrimPrefix(strings.TrimSuffix(registryURLB, "/"), "https://")
}
