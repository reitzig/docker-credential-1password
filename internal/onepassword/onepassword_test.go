package onepassword

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/1password/onepassword-sdk-go"
	"github.com/docker-credential-1password/internal/config"
)

type testClient struct {
	secrets map[config.AuthSecretRef]string
}

func (t *testClient) Secrets() onepassword.SecretsAPI {
	return testSecretsAPI{secrets: t.secrets}
}

type testSecretsAPI struct {
	secrets map[config.AuthSecretRef]string
}

func (t testSecretsAPI) lookup(secretReference string) (struct {
	config.AuthSecretRef
	string
}, error) {
	for ref, secret := range t.secrets {
		if ref.URI() == secretReference {
			return struct {
				config.AuthSecretRef
				string
			}{ref, secret}, nil
		}
	}
	return struct {
		config.AuthSecretRef
		string
	}{}, errors.New("secret not found")
}

func (t testSecretsAPI) Resolve(_ context.Context, secretReference string) (string, error) {
	result, err := t.lookup(secretReference)
	//goland:noinspection GoDfaErrorMayBeNotNil
	return result.string, err
}

func (t testSecretsAPI) ResolveAll(_ context.Context, secretReferences []string) (onepassword.ResolveAllResponse, error) {
	responses := make(map[string]onepassword.Response[onepassword.ResolvedReference, onepassword.ResolveReferenceError], len(secretReferences))

	for _, ref := range secretReferences {
		result, err := t.lookup(ref)
		if err != nil {
			responses[ref] = onepassword.Response[onepassword.ResolvedReference, onepassword.ResolveReferenceError]{
				Content: nil,
				Error:   &onepassword.ResolveReferenceError{Type: onepassword.ResolveReferenceErrorTypeVariantFieldNotFound},
			}
		} else {
			responses[ref] = onepassword.Response[onepassword.ResolvedReference, onepassword.ResolveReferenceError]{
				Content: &onepassword.ResolvedReference{
					Secret:  result.string,
					ItemID:  result.Item,
					VaultID: result.Vault,
				},
				Error: nil,
			}
		}
	}

	return onepassword.ResolveAllResponse{IndividualResponses: responses}, nil
}

func TestOpClient_ListUsers(t *testing.T) {
	type fields struct {
		config    config.Config
		sdkClient sdkClient
	}
	tests := []struct {
		name    string
		fields  fields
		want    RegistriesAndUsernames
		wantErr bool
	}{
		{
			name: "some registries configured",
			fields: fields{
				config: config.Config{
					Account: "irrelevant",
					SecretRefs: map[string]config.AuthSecretRefs{
						"https://some.registry.com": {
							Username: config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"},
							Secret:   config.AuthSecretRef{},
						},
						"https://my.registry.org": {
							Username: config.AuthSecretRef{Vault: "abcdef", Item: "My Hub", Field: "username"},
							Secret:   config.AuthSecretRef{},
						},
					},
				},
				sdkClient: &testClient{
					secrets: map[config.AuthSecretRef]string{
						config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"}:    "Some User",
						config.AuthSecretRef{Vault: "abcdef", Item: "My Hub", Field: "username"}: "My User",
					},
				},
			},
			want: map[string]string{
				"https://my.registry.org":   "My User",
				"https://some.registry.com": "Some User",
			},
		},
		{
			name: "some secret does not resolve",
			fields: fields{
				config: config.Config{
					Account: "irrelevant",
					SecretRefs: map[string]config.AuthSecretRefs{
						"https://some.registry.com": {
							Username: config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"},
							Secret:   config.AuthSecretRef{},
						},
						"https://my.registry.org": {
							Username: config.AuthSecretRef{Vault: "abcdef", Item: "My Hub", Field: "username"},
							Secret:   config.AuthSecretRef{},
						},
					},
				},
				sdkClient: &testClient{
					secrets: map[config.AuthSecretRef]string{
						config.AuthSecretRef{Vault: "abcdef", Item: "My Hub", Field: "username"}: "My User",
					},
				},
			},
			want: map[string]string{
				"https://my.registry.org": "My User",
			},
		},
		{
			name: "no registries configured",
			fields: fields{
				config: config.Config{
					Account:    "irrelevant",
					SecretRefs: map[string]config.AuthSecretRefs{},
				},
				sdkClient: &testClient{
					secrets: nil,
				},
			},
			want: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &OpClient{
				config:    tt.fields.config,
				sdkClient: tt.fields.sdkClient,
			}
			got, err := client.ListUsers()
			if (err != nil) != tt.wantErr {
				t.Errorf("ListUsers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListUsers() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpClient_AuthFor(t *testing.T) {
	type fields struct {
		config    config.Config
		sdkClient sdkClient
	}
	type args struct {
		registry string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    Auth
		wantErr bool
	}{
		{
			name: "registry fully configured",
			fields: fields{
				config: config.Config{
					Account: "irrelevant",
					SecretRefs: map[string]config.AuthSecretRefs{
						"https://some.registry.com": {
							Username: config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"},
							Secret:   config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "token"},
						},
					},
				},
				sdkClient: &testClient{
					secrets: map[config.AuthSecretRef]string{
						config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"}:   "Some User",
						config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "token"}: "qwerty",
					},
				},
			},
			args: args{
				registry: "https://some.registry.com",
			},
			want: Auth{
				Username: "Some User",
				Secret:   "qwerty",
			},
			wantErr: false,
		},
		{
			name: "registry not configured",
			fields: fields{
				config: config.Config{
					Account:    "irrelevant",
					SecretRefs: map[string]config.AuthSecretRefs{},
				},
				sdkClient: &testClient{
					secrets: map[config.AuthSecretRef]string{},
				},
			},
			args: args{
				registry: "https://some.registry.com",
			},
			want:    Auth{},
			wantErr: true,
		},
		{
			name: "secret missing",
			fields: fields{
				config: config.Config{
					Account: "irrelevant",
					SecretRefs: map[string]config.AuthSecretRefs{
						"https://some.registry.com": {
							Username: config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"},
							Secret:   config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "token"},
						},
					},
				},
				sdkClient: &testClient{
					secrets: map[config.AuthSecretRef]string{
						config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"}: "Some User",
					},
				},
			},
			args: args{
				registry: "https://some.registry.com",
			},
			want:    Auth{},
			wantErr: true,
		},
		{
			name: "secret empty",
			fields: fields{
				config: config.Config{
					Account: "irrelevant",
					SecretRefs: map[string]config.AuthSecretRefs{
						"https://some.registry.com": {
							Username: config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"},
							Secret:   config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "token"},
						},
					},
				},
				sdkClient: &testClient{
					secrets: map[config.AuthSecretRef]string{
						config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"}:   "Some User",
						config.AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "token"}: "",
					},
				},
			},
			args: args{
				registry: "https://some.registry.com",
			},
			want:    Auth{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &OpClient{
				config:    tt.fields.config,
				sdkClient: tt.fields.sdkClient,
			}
			got, err := client.AuthFor(tt.args.registry)
			if (err != nil) != tt.wantErr {
				t.Errorf("AuthFor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AuthFor() got = %v, want %v", got, tt.want)
			}
		})
	}
}
