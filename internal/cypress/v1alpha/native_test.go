package v1alpha

import (
	"path/filepath"
	"reflect"
	"testing"

	"gotest.tools/v3/fs"
)

func TestConfigFromFile(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		want        Config
		wantErr     bool
	}{
		{
			name:        "Valid File - Empty",
			fileContent: `{}`,
			want:        Config{},
			wantErr:     false,
		},
		{
			name:        "Valid File - Integration folder",
			fileContent: `{"integrationFolder":"./e2e/integration"}`,
			want:        Config{IntegrationFolder: "./e2e/integration"},
			wantErr:     false,
		},
		{
			name:        "Invalid File",
			fileContent: `{"integrationFolder":"./e2e/integration}`,
			want:        Config{},
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := fs.NewDir(t, "cypress-config", fs.WithMode(0755),
				fs.WithFile("cypress.json", tt.fileContent, fs.WithMode(0644)))
			defer dir.Remove()

			tt.want.Path = filepath.Join(dir.Path(), "cypress.json")

			got, err := configFromFile(dir.Join("cypress.json"))
			if (err != nil) != tt.wantErr {
				t.Errorf("configFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("configFromFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}
