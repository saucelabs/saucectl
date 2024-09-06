package runtime

import (
	"testing"

	"gotest.tools/assert"
)

func TestSelectNode(t *testing.T) {
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
			name:     "invalid version contains non-numeric major, minor and patch",
			runtimes: runtimes,
			version:  "va.b.c",
			want:     "",
			wantErr:  "invalid node version va.b.c",
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
			assert.Equal(t, tc.want, got.Version)
			if err != nil {
				assert.Equal(t, tc.wantErr, err.Error())
			}
		})
	}
}
