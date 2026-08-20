package config

import (
	"context"
	"reflect"
	"testing"

	"github.com/1password/onepassword-sdk-go"
)

func TestAuthSecretRefsURI(t *testing.T) {
	tests := []struct {
		name string
		ref  AuthSecretRef
		want string
	}{
		{
			name: "simple",
			ref:  AuthSecretRef{Vault: "Vault", Item: "Item", Field: "username"},
			want: "op://Vault/Item/username",
		},
		{
			name: "spaces",
			ref:  AuthSecretRef{Vault: "My Vault", Item: "Other Item", Field: "API Token"},
			want: "op://My Vault/Other Item/API Token",
		},
		{
			name: "with section",
			ref:  AuthSecretRef{Vault: "My Vault", Item: "Other Item", Field: "Structuring Section/API Token"},
			want: "op://My Vault/Other Item/Structuring Section/API Token",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.URI(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("URI() = %v, want %v", got, tt.want)
			} else if err := onepassword.Secrets.ValidateSecretReference(context.Background(), got); err != nil {
				t.Errorf("ValidateSecretReference() = %v", err)
			}
		})
	}
}

func TestRegistryMatches(t *testing.T) {
	tests := []struct {
		name  string
		some  string
		other string
		want  bool
	}{
		{
			name:  "exact match",
			some:  "https://index.docker.io/v1",
			other: "https://index.docker.io/v1",
			want:  true,
		},
		{
			name:  "other path",
			some:  "https://index.docker.io/v1",
			other: "https://index.docker.io/v2",
			want:  false,
		},
		{
			name:  "trailing slash",
			some:  "https://index.docker.io/v1",
			other: "https://index.docker.io/v1/",
			want:  true,
		},
		{
			name:  "no protocol",
			some:  "https://index.docker.io/v1",
			other: "index.docker.io/v1",
			want:  true,
		},
		{
			name:  "other port",
			some:  "gitlab.some.org:4567",
			other: "gitlab.some.org:4568",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := registryMatches(tt.some, tt.other); got != tt.want {
				t.Errorf("registryMatches(%s, %s) = %v, want %v", tt.some, tt.other, got, tt.want)
			}
		})
	}
}
