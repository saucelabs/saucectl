package framework

import (
	"reflect"
	"testing"
)

func TestGitReleaseSegments(t *testing.T) {
	type args struct {
		m *Metadata
	}
	tests := []struct {
		name     string
		args     args
		wantOrg  string
		wantRepo string
		wantTag  string
		wantErr  bool
	}{
		{
			name: "the regular usecase",
			args: args{
				&Metadata{
					GitRelease: "sauce/this-is-spicy:v1",
				},
			},
			wantOrg:  "sauce",
			wantRepo: "this-is-spicy",
			wantTag:  "v1",
			wantErr:  false,
		},
		{
			name: "malformed",
			args: args{
				&Metadata{
					GitRelease: "totally random string",
				},
			},
			wantErr: true,
		},
		{
			name: "empty",
			args: args{
				&Metadata{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOrg, gotRepo, gotTag, err := GitReleaseSegments(tt.args.m)
			if (err != nil) != tt.wantErr {
				t.Errorf("GitReleaseSegments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOrg != tt.wantOrg {
				t.Errorf("GitReleaseSegments() gotOrg = %v, want %v", gotOrg, tt.wantOrg)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("GitReleaseSegments() gotRepo = %v, want %v", gotRepo, tt.wantRepo)
			}
			if gotTag != tt.wantTag {
				t.Errorf("GitReleaseSegments() gotTag = %v, want %v", gotTag, tt.wantTag)
			}
		})
	}
}

func TestHasPlatform(t *testing.T) {
	type args struct {
		m        Metadata
		platform string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "exact match",
			args: args{
				m: Metadata{
					FrameworkName: "playwright",
					Platforms:     []Platform{{PlatformName: "Windows 11"}},
				},
				platform: "Windows 11",
			},
			want: true,
		},
		{
			name: "case mismatch",
			args: args{
				m: Metadata{
					FrameworkName: "playwright",
					Platforms:     []Platform{{PlatformName: "Windows 11"}},
				},
				platform: "windows 11",
			},
			want: true,
		},
		{
			name: "does not have platform",
			args: args{
				m: Metadata{
					FrameworkName: "playwright",
					Platforms:     []Platform{{PlatformName: "Windows 11"}},
				},
				platform: "Windows Me",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPlatform(tt.args.m, tt.args.platform); got != tt.want {
				t.Errorf("HasPlatform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlatformNames(t *testing.T) {
	type args struct {
		platforms []Platform
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "happy path",
			args: args{[]Platform{
				{
					PlatformName: "Windows 11",
				},
				{
					PlatformName: "macOS 12",
				},
			}},
			want: []string{"Windows 11", "macOS 12"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlatformNames(tt.args.platforms); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PlatformNames() = %v, want %v", got, tt.want)
			}
		})
	}
}
