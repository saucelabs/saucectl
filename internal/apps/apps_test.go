package apps

import (
	"fmt"
	"gotest.tools/v3/fs"
	"path/filepath"
	"testing"
)

func TestValidate(t *testing.T) {
	dir := fs.NewDir(t, "xcuitest-config",
		fs.WithFile("test.ipa", "", fs.WithMode(0655)),
		fs.WithFile("test.zip", "", fs.WithMode(0655)))
	defer dir.Remove()
	appIPA := filepath.Join(dir.Path(), "test.ipa")
	appZIP := filepath.Join(dir.Path(), "test.zip")
	badAppIPA := filepath.Join(dir.Path(), "bad-test.ipa")

	type args struct {
		kind       string
		app        string
		validExt   []string
		URLAllowed bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "Valid application file",
			args: args{
				kind:       "application",
				app:        appIPA,
				validExt:   []string{".ipa"},
				URLAllowed: false,
			},
			wantErr: nil,
		},
		{
			name: "Supports storage link - format01",
			args: args{
				kind:       "application",
				app:        "storage:f8b9ed63-cea7-4fd3-8b18-d9ad7b71c11d",
				validExt:   []string{".ipa"},
				URLAllowed: false,
			},
			wantErr: nil,
		},
		{
			name: "Supports storage link - format02",
			args: args{
				kind:       "application",
				app:        "storage://f8b9ed63-cea7-4fd3-8b18-d9ad7b71c11d",
				validExt:   []string{".ipa"},
				URLAllowed: false,
			},
			wantErr: nil,
		},
		{
			name: "Non-existing file",
			args: args{
				kind:       "application",
				app:        badAppIPA,
				validExt:   []string{".ipa"},
				URLAllowed: false,
			},
			wantErr: fmt.Errorf("%s: file not found", badAppIPA),
		},
		{
			name: "Invalid file extension",
			args: args{
				kind:       "application",
				app:        appZIP,
				validExt:   []string{".ipa"},
				URLAllowed: false,
			},
			wantErr: fmt.Errorf("invalid application file: %s, make sure extension is one of the following: %s", appZIP, ".ipa"),
		},
		{
			name: "Bad storage id",
			args: args{
				kind:       "application",
				app:        "storage:bad-link",
				validExt:   []string{".ipa"},
				URLAllowed: false,
			},
			wantErr: fmt.Errorf("invalid application file: storage:bad-link, make sure extension is one of the following: .ipa"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.args.kind, tt.args.app, tt.args.validExt, tt.args.URLAllowed)
			if (err == nil && tt.wantErr != nil) ||
				(err != nil && tt.wantErr == nil) ||
				(err != nil && tt.wantErr.Error() != err.Error()) {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_hasValidExtension(t *testing.T) {
	type args struct {
		file string
		exts []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Have valid ext",
			args: args{
				file: "/var/tmp/app.ipa",
				exts: []string{".app", ".ipa"},
			},
			want: true,
		},
		{
			name: "Have no valid ext",
			args: args{
				file: "/var/tmp/app.apk",
				exts: []string{".app", ".ipa"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasValidExtension(tt.args.file, tt.args.exts); got != tt.want {
				t.Errorf("hasValidExtension() = %v, want %v", got, tt.want)
			}
		})
	}
}
