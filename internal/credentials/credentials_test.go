package credentials

import (
	"os"
	"reflect"
	"testing"
)

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name       string
		beforeTest func()
		want       Credentials
	}{
		{
			name: "env vars exist",
			beforeTest: func() {
				_ = os.Setenv("SAUCE_USERNAME", "saucebot")
				_ = os.Setenv("SAUCE_ACCESS_KEY", "123")
			},
			want: Credentials{
				Username:  "saucebot",
				AccessKey: "123",
				Source:    "environment variables",
			},
		},
		{
			name: "env vars don't exist",
			beforeTest: func() {
				_ = os.Unsetenv("SAUCE_USERNAME")
				_ = os.Unsetenv("SAUCE_ACCESS_KEY")
			},
			want: Credentials{
				Source: "environment variables",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.beforeTest()
			if got := FromEnv(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCredentials_IsValid(t *testing.T) {
	type fields struct {
		Username  string
		AccessKey string
		Source    string
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
			c := &Credentials{
				Username:  tt.fields.Username,
				AccessKey: tt.fields.AccessKey,
				Source:    tt.fields.Source,
			}
			if got := c.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
