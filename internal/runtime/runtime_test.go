package runtime

import (
	"testing"

	"gotest.tools/assert"
)

func TestFind(t *testing.T) {
	runtimes := []Runtime{
		{
			Name:    "nodejs",
			Version: "20.14.0",
			Alias:   []string{"iron", "lts"},
		},
		{
			Name:    "nodejs",
			Version: "18.20.4",
			Alias:   []string{"Hydrogen"},
		},
	}
	testcases := []struct {
		caseName string
		runtimes []Runtime
		name     string
		version  string
		want     string
		wantErr  string
	}{
		{
			caseName: "version is invalid",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "vfake-version",
			want:     "",
			wantErr:  "invalid Node.js version vfake-version",
		},
		{
			caseName: "version alias is invalid",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "my-alias",
			want:     "",
			wantErr:  "invalid Node.js version my-alias",
		},
		{
			caseName: "version alias is valid",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "iron",
			want:     "20.14.0",
			wantErr:  "",
		},
		{
			caseName: "valid version contains major, minor and patch",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v20.14.0",
			want:     "20.14.0",
			wantErr:  "",
		},
		{
			caseName: "invalid version not starts with v",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "20.14.0",
			want:     "",
			wantErr:  "invalid Node.js version 20.14.0",
		},
		{
			caseName: "invalid version contains major, minor and patch",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v20.14.2",
			want:     "",
			wantErr:  "no matching Node.js version found for v20.14.2",
		},
		{
			caseName: "invalid version contains non-numeric major, minor and patch",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "va.b.c",
			want:     "",
			wantErr:  "invalid Node.js version va.b.c",
		},
		{
			caseName: "valid version only contains major and minor",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v20.14",
			want:     "20.14.0",
			wantErr:  "",
		},
		{
			caseName: "valid version only contains major",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v18",
			want:     "18.20.4",
			wantErr:  "",
		},
		{
			caseName: "invalid version only contains major",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v22",
			want:     "",
			wantErr:  "no matching Node.js version found for v22",
		},
		{
			caseName: "should precisely match complete major and minor version",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v20.1",
			want:     "",
			wantErr:  "no matching Node.js version found for v20.1",
		},
		{
			caseName: "should precisely match complete major version",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v2",
			want:     "",
			wantErr:  "no matching Node.js version found for v2",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.caseName, func(t *testing.T) {
			got, err := Find(tc.runtimes, tc.name, tc.version)
			assert.Equal(t, tc.want, got.Version)
			if err != nil {
				assert.Equal(t, tc.wantErr, err.Error())
			}
		})
	}
}
