package ci

import (
	"os"
	"testing"
)

func TestAvailable(t *testing.T) {
	tests := []struct {
		name     string
		envSetup func()
		want     bool
	}{
		{
			name: "detect CI",
			envSetup: func() {
				os.Setenv("CI", "1")
			},
			want: true,
		},
		{
			name: "detect build identifier",
			envSetup: func() {
				os.Setenv("BUILD_NUMBER", "123")
			},
			want: true,
		},
		{
			name: "detect nothing",
			envSetup: func() {
				os.Unsetenv("CI")
				os.Unsetenv("BUILD_NUMBER")
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.envSetup()
			if got := IsAvailable(); got != tt.want {
				t.Errorf("Available() = %v, want %v", got, tt.want)
			}
		})
	}
}
