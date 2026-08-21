package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker-credential-1password/internal/app"
	"github.com/docker-credential-1password/internal/config"
	"github.com/docker-credential-1password/internal/onepassword"
)

type DockerAuth struct {
	Username string
	Secret   string
}

func readInputLine() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func main() {
	if len(os.Args) > 1 {
		switch command := os.Args[1]; command {
		case "get":
			registryURL, err := readInputLine()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to read registry URL from stdin: %v\n", err)
				os.Exit(1)
			}

			settings, err := config.Load()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
				os.Exit(2)
			}

			client, err := onepassword.Client(settings)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to set up 1Password client: %v\n", err)
				os.Exit(3)
			}

			auth, err := client.AuthFor(registryURL)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Could not retrieve credentials for '%s': %v\n", registryURL, err)
				os.Exit(4)
			}

			data, err := json.Marshal(DockerAuth{Username: auth.Username, Secret: auth.Secret})
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to encode auth as JSON: %v\n", err)
				os.Exit(5)
			}

			fmt.Println(string(data))

		case "list":
			settings, err := config.Load()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
				os.Exit(2)
			}

			client, err := onepassword.Client(settings)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to set up 1Password client: %v\n", err)
				os.Exit(3)
			}

			names, err := client.ListUsers()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Could not retrieve usernames': %v\n", err)
				os.Exit(4)
			}

			data, err := json.Marshal(names)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to encode usernames as JSON: %v\n", err)
				os.Exit(5)
			}

			fmt.Println(string(data))

		case "version":
			fmt.Printf("%s\n\nVersion: %s\nCommit: %s\nDate: %s\n", app.Name, app.Version, app.Commit, app.Date)

		case "store", "erase":
			_, _ = fmt.Fprintf(os.Stderr, "Command '%s' not implemented; manage secrets through 1Password\n", command)
			os.Exit(6)
		default:
			_, _ = fmt.Fprintf(os.Stderr, "Unknown command '%s'; use 'get','list', 'version'\n", command)
			os.Exit(6)
		}
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "No command given; use 'get', 'list', 'version'\n")
		os.Exit(7)
	}
}
