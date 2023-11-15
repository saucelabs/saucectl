package credentials

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/region"
)

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name       string
		beforeTest func()
		want       iam.Credentials
	}{
		{
			name: "env vars exist",
			beforeTest: func() {
				_ = os.Setenv("SAUCE_USERNAME", "saucebot")
				_ = os.Setenv("SAUCE_ACCESS_KEY", "123")
			},
			want: iam.Credentials{
				Username:  "saucebot",
				AccessKey: "123",
				Source:    "Environment variables($SAUCE_USERNAME, $SAUCE_ACCESS_KEY)",
			},
		},
		{
			name: "env vars don't exist",
			beforeTest: func() {
				_ = os.Unsetenv("SAUCE_USERNAME")
				_ = os.Unsetenv("SAUCE_ACCESS_KEY")
			},
			want: iam.Credentials{Source: "Environment variables($SAUCE_USERNAME, $SAUCE_ACCESS_KEY)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.beforeTest()
			if got := FromEnv(); !cmp.Equal(got, tt.want) {
				t.Errorf("FromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCredentials_IsValid(t *testing.T) {
	type fields struct {
		Username  string
		AccessKey string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "all set",
			fields: fields{
				Username:  "saucebot",
				AccessKey: "123",
			},
			want: true,
		},
		{
			name: "username is missing",
			fields: fields{
				Username:  "",
				AccessKey: "123",
			},
			want: false,
		},
		{
			name: "access key is missing",
			fields: fields{
				Username:  "saucebot",
				AccessKey: "",
			},
			want: false,
		},
		{
			name:   "everything is missing",
			fields: fields{},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &iam.Credentials{
				Username:  tt.fields.Username,
				AccessKey: tt.fields.AccessKey,
			}
			if got := c.IsSet(); got != tt.want {
				t.Errorf("IsSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromFile(t *testing.T) {
	// put everything in safe location we can clean up later
	tempDir, err := os.MkdirTemp("", "saucectl-creds-test")
	if err != nil {
		t.Errorf("Failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	type args struct {
		path string
	}
	tests := []struct {
		name       string
		args       args
		region     region.Region
		beforeTest func()
		want       iam.Credentials
	}{
		{
			name: "creds exist",
			args: args{
				path: filepath.Join(tempDir, "credilicious.yml"),
			},
			region: region.None,
			beforeTest: func() {
				c := iam.Credentials{
					Username:  "saucebot",
					AccessKey: "123",
				}
				if err := toFile(c, filepath.Join(tempDir, "credilicious.yml")); err != nil {
					t.Errorf("Failed to create credentials file: %v", err)
				}
			},
			want: iam.Credentials{
				Username:  "saucebot",
				AccessKey: "123",
			},
		},
		{
			name: "creds don't exist",
			args: args{
				path: filepath.Join(tempDir, "you-shall-not-find-me.yml"),
			},
			beforeTest: func() {},
			want:       iam.Credentials{},
		},
		{
			name: "multiple creds exist, selected by specified region",
			args: args{
				path: filepath.Join(tempDir, "credilicious.yml"),
			},
			region: region.Staging,
			beforeTest: func() {
				c := iam.Credentials{
					Username:  "saucebot",
					AccessKey: "123",
					Regional: []iam.Credentials{
						{
							Username:  "saucebot-staging",
							AccessKey: "123-staging",
							Region:    "staging",
						},
					},
				}
				if err := toFile(c, filepath.Join(tempDir, "credilicious.yml")); err != nil {
					t.Errorf("Failed to create credentials file: %v", err)
				}
			},
			want: iam.Credentials{
				Username:  "saucebot-staging",
				AccessKey: "123-staging",
			},
		},
		{
			name: "multiple creds exist, return default when no region set",
			args: args{
				path: filepath.Join(tempDir, "credilicious.yml"),
			},
			region: region.None,
			beforeTest: func() {
				c := iam.Credentials{
					Username:  "saucebot",
					AccessKey: "123",
					Regional: []iam.Credentials{
						{
							Username:  "saucebot-staging",
							AccessKey: "123-staging",
							Region:    "staging",
						},
					},
				}
				if err := toFile(c, filepath.Join(tempDir, "credilicious.yml")); err != nil {
					t.Errorf("Failed to create credentials file: %v", err)
				}
			},
			want: iam.Credentials{
				Username:  "saucebot",
				AccessKey: "123",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.beforeTest()
			got := fromFile(tt.args.path, tt.region)
			if !cmp.Equal(got.Username, tt.want.Username) || !cmp.Equal(got.AccessKey, tt.want.AccessKey) {
				t.Errorf("FromFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_defaultFilepath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Errorf("Unable to determine home directory: %v", err)
	}

	tests := []struct {
		name string
		want string
	}{
		{
			name: "a file at home",
			want: filepath.Join(home, ".sauce", "credentials.yml"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultCredsPath; got != tt.want {
				t.Errorf("defaultFilepath() = %v, want %v", got, tt.want)
			}
		})
	}
}
