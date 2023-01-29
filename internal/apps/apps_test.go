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
		kind     string
		app      string
		validExt []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "Valid application file",
			args: args{
				kind:     "application",
				app:      appIPA,
				validExt: []string{".ipa"},
			},
			wantErr: nil,
		},
		{
			name: "Supports storage link - format01",
			args: args{
				kind:     "application",
				app:      "storage:f8b9ed63-cea7-4fd3-8b18-d9ad7b71c11d",
				validExt: []string{".ipa"},
			},
			wantErr: nil,
		},
		{
			name: "Supports storage link - format02",
			args: args{
				kind:     "application",
				app:      "storage://f8b9ed63-cea7-4fd3-8b18-d9ad7b71c11d",
				validExt: []string{".ipa"},
			},
			wantErr: nil,
		},
		{
			name: "Non-existing file",
			args: args{
				kind:     "application",
				app:      badAppIPA,
				validExt: []string{".ipa"},
			},
			wantErr: fmt.Errorf("%s: file not found", badAppIPA),
		},
		{
			name: "Invalid file extension",
			args: args{
				kind:     "application",
				app:      appZIP,
				validExt: []string{".ipa"},
			},
			wantErr: fmt.Errorf("invalid application file: %s, make sure extension is one of the following: %s", appZIP, ".ipa"),
		},
		{
			name: "Bad storage id",
			args: args{
				kind:     "application",
				app:      "storage:bad-link",
				validExt: []string{".ipa"},
			},
			wantErr: fmt.Errorf("invalid application file: storage:bad-link, make sure extension is one of the following: .ipa"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.args.kind, tt.args.app, tt.args.validExt)
			if (err == nil && tt.wantErr != nil) ||
				(err != nil && tt.wantErr == nil) ||
				(err != nil && tt.wantErr.Error() != err.Error()) {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsRemote(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "Valid remote url",
			filename: "http://a.file.to/download",
			want:     true,
		},
		{
			name:     "case insensitive scheme",
			filename: "HTTP://a.file.to/download",
			want:     true,
		},
		{
			name:     "Local file url",
			filename: "file://a.file.to/download",
			want:     false,
		},
		{
			name:     "http prefixed filename",
			filename: "httpApplication.ipa",
			want:     false,
		},
		{
			name:     "local absolute filepath",
			filename: "/a/local/path",
			want:     false,
		},
		{
			name:     "local relative filepath",
			filename: "./a/local/path",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRemote(tt.filename); got != tt.want {
				t.Errorf("IsRemote() = %v, want %v", got, tt.want)
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

func TestIsStorageReference(t *testing.T) {
	type args struct {
		link string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Simple ID",
			args: args{
				link: "f4d71508-bec6-4db9-9694-d2d028db6cef",
			},
			want: true,
		},
		{
			name: "Storage Prefix + ID",
			args: args{
				link: "storage:f4d71508-bec6-4db9-9694-d2d028db6cef",
			},
			want: true,
		},
		{
			name: "Storage Prefix :// + ID",
			args: args{
				link: "storage://f4d71508-bec6-4db9-9694-d2d028db6cef",
			},
			want: true,
		},
		{
			name: "Filename IPA",
			args: args{
				link: "storage:filename=dummyfilename.ipa",
			},
			want: true,
		},
		{
			name: "Filename APK",
			args: args{
				link: "storage:filename=dummyfilename.apk",
			},
			want: true,
		},
		{
			name: "Filename ZIP",
			args: args{
				link: "storage:filename=dummyfilename.zip",
			},
			want: false,
		},
		{
			name: "Bad Reference",
			args: args{
				link: "storage:bad-ref",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStorageReference(tt.args.link); got != tt.want {
				t.Errorf("IsStorageReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStandardizeReferenceLink(t *testing.T) {
	tests := []struct {
		name       string
		storageRef string
		want       string
	}{
		{
			name:       "Only ID",
			storageRef: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			want:       "storage:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		{
			name:       "storage:ID",
			storageRef: "storage:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			want:       "storage:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		{
			name:       "storage://ID",
			storageRef: "storage://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			want:       "storage:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		{
			name:       "storage:filename=dummy",
			storageRef: "storage:filename=dummy",
			want:       "storage:filename=dummy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StandardizeReferenceLink(tt.storageRef); got != tt.want {
				t.Errorf("StandardizeReferenceLink() = %v, want %v", got, tt.want)
			}
		})
	}
}
