package framework

import (
	"reflect"
	"testing"

	"gotest.tools/v3/assert"
)

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

func TestSelectNode(t *testing.T) {
	runtimes := []Runtime{
		{
			RuntimeName:    "nodejs",
			RuntimeVersion: "20.14.0",
			RuntimeAlias:   []string{"iron", "lts"},
		},
		{
			RuntimeName:    "nodejs",
			RuntimeVersion: "18.20.4",
			RuntimeAlias:   []string{"Hydrogen"},
		},
	}
	testcases := []struct {
		name     string
		runtimes []Runtime
		version  string
		want     string
		wantErr  string
	}{
		{
			name:     "version is invalid",
			runtimes: runtimes,
			version:  "vfake-version",
			want:     "",
			wantErr:  "invalid node version vfake-version",
		},
		{
			name:     "version alias is invalid",
			runtimes: runtimes,
			version:  "my-alias",
			want:     "",
			wantErr:  "invalid node version my-alias",
		},
		{
			name:     "version alias is valid",
			runtimes: runtimes,
			version:  "iron",
			want:     "20.14.0",
			wantErr:  "",
		},
		{
			name:     "valid version contains major, minor and patch",
			runtimes: runtimes,
			version:  "v20.14.0",
			want:     "20.14.0",
			wantErr:  "",
		},
		{
			name:     "invalid version not starts with v",
			runtimes: runtimes,
			version:  "20.14.0",
			want:     "",
			wantErr:  "invalid node version 20.14.0",
		},
		{
			name:     "invalid version contains major, minor and patch",
			runtimes: runtimes,
			version:  "v20.14.2",
			want:     "",
			wantErr:  "no matching node version found for v20.14.2",
		},
		{
			name:     "valid version only contains major and minor",
			runtimes: runtimes,
			version:  "v20.14",
			want:     "20.14.0",
			wantErr:  "",
		},
		{
			name:     "valid version only contains major",
			runtimes: runtimes,
			version:  "v18",
			want:     "18.20.4",
			wantErr:  "",
		},
		{
			name:     "invalid version only contains major",
			runtimes: runtimes,
			version:  "v22",
			want:     "",
			wantErr:  "no matching node version found for v22",
		},
		{
			name:     "should precisely match complete major and minor version",
			runtimes: runtimes,
			version:  "v20.1",
			want:     "",
			wantErr:  "no matching node version found for v20.1",
		},
		{
			name:     "should precisely match complete major version",
			runtimes: runtimes,
			version:  "v2",
			want:     "",
			wantErr:  "no matching node version found for v2",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SelectNode(tc.runtimes, tc.version)
			assert.Equal(t, tc.want, got.RuntimeVersion)
			if err != nil {
				assert.Equal(t, tc.wantErr, err.Error())
			}
		})
	}
}
