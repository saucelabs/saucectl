package cypress

import (
	"gotest.tools/v3/fs"
	"reflect"
	"testing"
)

func TestConfigFromFile(t *testing.T) {
	type args struct {
		fileName string
	}
	tests := []struct {
		name        string
		fileContent string
		want        Config
		wantErr     bool
	}{
		{
			name: "Valid File - Empty",
			fileContent: `{}`,
			want: Config{},
			wantErr: false,
		},
		{
			name: "Valid File - Integration folder",
			fileContent: `{"integrationFolder":"./e2e/integration"}`,
			want: Config{IntegrationFolder: "./e2e/integration"},
			wantErr: false,
		},
		{
			name: "Invalid File",
			fileContent: `{"integrationFolder":"./e2e/integration}`,
			want: Config{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := fs.NewDir(t, "cypress-config", fs.WithMode(0755),
				fs.WithFile("cypress.json", tt.fileContent, fs.WithMode(0644)))
			defer dir.Remove()
			got, err := ConfigFromFile(dir.Join("cypress.json"))
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConfigFromFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}
