package config

import (
	"context"
	"fmt"
	"os"
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

func TestConfig_RefsFor(t *testing.T) {
	type fields struct {
		Account    string
		SecretRefs map[string]AuthSecretRefs
	}
	type args struct {
		registry string
	}
	targetRefs := AuthSecretRefs{
		Username: AuthSecretRef{Vault: "abcdef", Item: "My Hub", Field: "username"},
		Secret:   AuthSecretRef{Vault: "abcdef", Item: "My Registry", Field: "password"},
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    AuthSecretRefs
		wantErr bool
	}{
		{
			name: "exact match configured",
			fields: fields{
				Account: "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{
					"https://some.registry.com":  {},
					"https://my.registry.org":    targetRefs,
					"https://other.registry.com": {},
				},
			},
			args:    args{registry: "https://my.registry.org"},
			want:    targetRefs,
			wantErr: false,
		},
		{
			name: "exact match up to protocol configured",
			fields: fields{
				Account: "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{
					"https://some.registry.com":  {},
					"https://my.registry.org":    targetRefs,
					"https://other.registry.com": {},
				},
			},
			args:    args{registry: "my.registry.org"},
			want:    targetRefs,
			wantErr: false,
		},
		{
			name: "match up to path configured",
			fields: fields{
				Account: "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{
					"some.registry.com":  {},
					"my.registry.org":    targetRefs,
					"other.registry.com": {},
				},
			},
			args:    args{registry: "https://my.registry.org/v1"},
			want:    targetRefs,
			wantErr: false,
		},
		{
			name: "match up to subdomain configured",
			fields: fields{
				Account: "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{
					"some.registry.com":  {},
					"my.registry.org":    targetRefs,
					"other.registry.com": {},
				},
			},
			args:    args{registry: "https://special.my.registry.org"},
			want:    targetRefs,
			wantErr: false,
		},
		{
			name: "match up to port configured",
			fields: fields{
				Account: "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{
					"some.registry.com":  {},
					"my.registry.org":    targetRefs,
					"other.registry.com": {},
				},
			},
			args:    args{registry: "my.registry.org:4567"},
			want:    targetRefs,
			wantErr: false,
		},
		{
			name: "multiple potential matches configured",
			fields: fields{
				Account: "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{
					"some.registry.com":          {},
					"my.registry.org":            {},
					"special.my.registry.org/v1": {},
					"my.registry.org/v1":         targetRefs,
					"registry.org":               {},
					"other.registry.com":         {},
				},
			},
			args:    args{registry: "https://my.registry.org/v1/"},
			want:    targetRefs,
			wantErr: false,
		},
		{
			name: "registry missing",
			fields: fields{
				Account: "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{
					"https://some.registry.com":  {},
					"https://other.registry.com": {},
				},
			},
			args:    args{registry: "my.registry.org"},
			want:    AuthSecretRefs{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Account:    tt.fields.Account,
				SecretRefs: tt.fields.SecretRefs,
			}
			got, err := config.RefsFor(tt.args.registry)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefsFor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RefsFor() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_filePath(t *testing.T) {
	tests := []struct {
		name            string
		envHome         *string
		envDockerConfig *string
		want            string
		wantErr         bool
	}{
		{
			name:            "default",
			envHome:         new("/home/peon"),
			envDockerConfig: nil,
			want:            "/home/peon/.docker/credential-1password.json",
			wantErr:         false,
		},
		{
			name:            "custom docker config dir",
			envHome:         new("/home/peon"),
			envDockerConfig: new("/tmp/foo"),
			want:            "/tmp/foo/credential-1password.json",
			wantErr:         false,
		},
		{
			name:            "blank custom dir",
			envHome:         new("/home/peon"),
			envDockerConfig: new(""),
			want:            "/home/peon/.docker/credential-1password.json",
			wantErr:         false,
		},
		{
			name:            "no config dir, no home",
			envHome:         nil,
			envDockerConfig: nil,
			want:            "",
			wantErr:         true,
		},
	}

	previousHome := os.Getenv("HOME")
	defer func(key, value string) {
		err := os.Setenv(key, value)
		if err != nil {
			fmt.Println("Resetting HOME environment variable failed")
		}
	}("HOME", previousHome)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envHome != nil {
				_ = os.Setenv("HOME", *tt.envHome)
			} else {
				_ = os.Unsetenv("HOME")
			}
			if tt.envDockerConfig != nil {
				_ = os.Setenv("DOCKER_CONFIG", *tt.envDockerConfig)
			} else {
				_ = os.Unsetenv("DOCKER_CONFIG")
			}

			got, err := filePath()
			if (err != nil) != tt.wantErr {
				t.Errorf("filePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("filePath() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_UsernameRefs(t *testing.T) {
	type fields struct {
		Account    string
		SecretRefs map[string]AuthSecretRefs
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]string
	}{
		{
			name: "some registries configured",
			fields: fields{
				Account: "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{
					"https://some.registry.com": {
						Username: AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"},
						Secret:   AuthSecretRef{},
					},
					"https://my.registry.org": {
						Username: AuthSecretRef{Vault: "abcdef", Item: "My Hub", Field: "username"},
						Secret:   AuthSecretRef{},
					},
				},
			},
			want: map[string]string{
				"https://some.registry.com": "op://abcdef/Some Hub/uid",
				"https://my.registry.org":   "op://abcdef/My Hub/username",
			},
		},
		{
			name: "no registries configured",
			fields: fields{
				Account:    "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{},
			},
			want: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Account:    tt.fields.Account,
				SecretRefs: tt.fields.SecretRefs,
			}
			if got := config.UsernameRefs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UsernameRefs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_RegistryNames(t *testing.T) {
	type fields struct {
		Account    string
		SecretRefs map[string]AuthSecretRefs
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "some registries configured",
			fields: fields{
				Account: "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{
					"https://some.registry.com": {
						Username: AuthSecretRef{Vault: "abcdef", Item: "Some Hub", Field: "uid"},
						Secret:   AuthSecretRef{},
					},
					"https://my.registry.org": {
						Username: AuthSecretRef{Vault: "abcdef", Item: "My Hub", Field: "username"},
						Secret:   AuthSecretRef{},
					},
				},
			},
			want: []string{
				"https://my.registry.org",
				"https://some.registry.com",
			},
		},
		{
			name: "no registries configured",
			fields: fields{
				Account:    "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{},
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Account:    tt.fields.Account,
				SecretRefs: tt.fields.SecretRefs,
			}
			if got := config.RegistryNames(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RegistryNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_AccountName(t *testing.T) {
	type fields struct {
		Account    string
		SecretRefs map[string]AuthSecretRefs
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "config set up",
			fields: fields{
				Account:    "irrelevant",
				SecretRefs: map[string]AuthSecretRefs{},
			},
			want:    "irrelevant",
			wantErr: false,
		},
		{
			name:    "config empty",
			fields:  fields{},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Account:    tt.fields.Account,
				SecretRefs: tt.fields.SecretRefs,
			}
			got, err := config.AccountName()
			if (err != nil) != tt.wantErr {
				t.Errorf("AccountName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("AccountName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_normalizeRegistry(t *testing.T) {
	type args struct {
		registryURL string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "already normalized",
			args: args{
				registryURL: "some.registry.com",
			},
			want: "some.registry.com",
		},
		{
			name: "complicated but normalized",
			args: args{
				registryURL: "some.registry.com:4567/v1",
			},
			want: "some.registry.com:4567/v1",
		},
		{
			name: "spurious protocol",
			args: args{
				registryURL: "https://some.registry.com",
			},
			want: "some.registry.com",
		},
		{
			name: "other spurious protocol",
			args: args{
				registryURL: "oci://some.registry.com",
			},
			want: "some.registry.com",
		},
		{
			name: "spurious trailing slash",
			args: args{
				registryURL: "some.registry.com/",
			},
			want: "some.registry.com",
		},
		{
			name: "spurious whitespace",
			args: args{
				registryURL: " some.registry.com/ ",
			},
			want: "some.registry.com",
		},
		{
			name: "spurious everything",
			args: args{
				registryURL: "http://some.registry.com// ",
			},
			want: "some.registry.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRegistry(tt.args.registryURL); got != tt.want {
				t.Errorf("normalizeRegistry() = %v, want %v", got, tt.want)
			}
		})
	}
}
