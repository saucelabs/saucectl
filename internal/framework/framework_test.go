package framework

import (
	"reflect"
	"testing"
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
