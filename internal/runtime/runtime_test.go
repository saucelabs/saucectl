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
		wantErr  bool
	}{
		{
			caseName: "version is invalid",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "vfake-version",
			want:     "",
			wantErr:  false,
		},
		{
			caseName: "version alias is invalid",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "my-alias",
			want:     "",
			wantErr:  false,
		},
		{
			caseName: "version alias is valid",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "iron",
			want:     "20.14.0",
			wantErr:  true,
		},
		{
			caseName: "valid version contains major, minor and patch",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v20.14.0",
			want:     "20.14.0",
			wantErr:  true,
		},
		{
			caseName: "invalid version not starts with v",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "20.14.0",
			want:     "",
			wantErr:  false,
		},
		{
			caseName: "invalid version contains major, minor and patch",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v20.14.2",
			want:     "",
			wantErr:  false,
		},
		{
			caseName: "invalid version contains non-numeric major, minor and patch",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "va.b.c",
			want:     "",
			wantErr:  false,
		},
		{
			caseName: "valid version only contains major and minor",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v20.14",
			want:     "20.14.0",
			wantErr:  true,
		},
		{
			caseName: "valid version only contains major",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v18",
			want:     "18.20.4",
			wantErr:  true,
		},
		{
			caseName: "invalid version only contains major",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v22",
			want:     "",
			wantErr:  false,
		},
		{
			caseName: "should precisely match complete major and minor version",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v20.1",
			want:     "",
			wantErr:  false,
		},
		{
			caseName: "should precisely match complete major version",
			runtimes: runtimes,
			name:     NodeRuntime,
			version:  "v2",
			want:     "",
			wantErr:  false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.caseName, func(t *testing.T) {
			got, err := Find(tc.runtimes, tc.name, tc.version)
			if (err != nil) == tc.wantErr {
				t.Errorf("Find() got = %v, want err", err)
			}
			assert.Equal(t, tc.want, got.Version)
		})
	}
}
