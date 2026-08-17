package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/1password/onepassword-sdk-go"
)

func TestAuthSecretRefs_asUris(t *testing.T) {
	type fields struct {
		vault string
		item  string
		field string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "simple",
			fields: fields{vault: "Vault", item: "Item", field: "username"},
			want:   "op://Vault/Item/username",
		},
		{
			name:   "spaces",
			fields: fields{vault: "My Vault", item: "Other Item", field: "API Token"},
			want:   "op://My Vault/Other Item/API Token",
		},
		{
			name:   "with section",
			fields: fields{vault: "My Vault", item: "Other Item", field: "Structuring Section/API Token"},
			want:   "op://My Vault/Other Item/Structuring Section/API Token",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := AuthSecretRef{
				Vault: tt.fields.vault,
				Item:  tt.fields.item,
				Field: tt.fields.field,
			}
			if got := r.asURI(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("asUris() = %v, want %v", got, tt.want)
			} else {
				err := onepassword.Secrets.ValidateSecretReference(context.Background(), got)
				if err != nil {
					t.Errorf("ValidateSecretReference() = %v", err)
				}
			}
		})
	}
}

func TestRegistryMatch(t *testing.T) {
	type fields struct {
		some  string
		other string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name:   "exact match",
			fields: fields{some: "https://index.docker.io/v1", other: "https://index.docker.io/v1"},
			want:   true,
		},
		{
			name:   "other path",
			fields: fields{some: "https://index.docker.io/v1", other: "https://index.docker.io/v2"},
			want:   false,
		},
		{
			name:   "trailing slash",
			fields: fields{some: "https://index.docker.io/v1", other: "https://index.docker.io/v1/"},
			want:   true,
		},
		{
			name:   "no protocol",
			fields: fields{some: "https://index.docker.io/v1", other: "index.docker.io/v1"},
			want:   true,
		},
		{
			name:   "other port",
			fields: fields{some: "gitlab.some.org:4567", other: "gitlab.some.org:4568"},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := registryMatches(tt.fields.some, tt.fields.other); got != tt.want {
				t.Errorf("registryMatches(%s, %s) = %v, want %v", tt.fields.some, tt.fields.other, got, tt.want)
			}
		})
	}
}
