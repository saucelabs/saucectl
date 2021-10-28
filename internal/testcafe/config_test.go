package testcafe

import (
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
	"os"
	"reflect"
	"testing"
)

func Test_appleDeviceRegex(t *testing.T) {
	tests := []struct {
		deviceName string
		want       bool
	}{
		{
			deviceName: "iPhone Simulator",
			want:       true,
		},
		{
			deviceName: "iphone simulator",
			want:       true,
		},
		{
			deviceName: "iPhone SE (2nd generation) Simulator",
			want:       true,
		},
		{
			deviceName: "iPhone 8 Simulator",
			want:       true,
		},
		{
			deviceName: "iPhone 8 Plus Simulator",
			want:       true,
		},
		{
			deviceName: "iPad Pro (12.9 inch) Simulator",
			want:       true,
		},
		{
			deviceName: "iPad Pro (12.9 inch) (4th generation) Simulator",
			want:       true,
		},
		{
			deviceName: "iPad Air Simulator",
			want:       true,
		},
		{
			deviceName: "iPad (8th generation) Simulator",
			want:       true,
		},
		{
			deviceName: "iPad mini (5th generation) Simulator",
			want:       true,
		},
		{
			deviceName: "Android GoogleAPI Emulator",
			want:       false,
		},
		{
			deviceName: "Google Pixel 3a GoogleAPI Emulator",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.deviceName, func(t *testing.T) {
			got := appleDeviceRegex.MatchString(tt.deviceName)
			if got != tt.want {
				t.Errorf("appleDeviceRegex.MatchString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterSuites(t *testing.T) {
	testCase := []struct {
		name      string
		config    *Project
		suiteName string
		expConfig Project
		expErr    string
	}{
		{
			name: "filtered suite exists in config",
			config: &Project{Suites: []Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			suiteName: "suite1",
			expConfig: Project{Suites: []Suite{
				{
					Name: "suite1",
				},
			}},
		},
		{
			name: "filtered suite does not exist in config",
			config: &Project{Suites: []Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			suiteName: "suite3",
			expConfig: Project{Suites: []Suite{
				{
					Name: "suite1",
				},
				{
					Name: "suite2",
				},
			}},
			expErr: "no suite named 'suite3' found",
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			err := FilterSuites(tc.config, tc.suiteName)
			if err != nil {
				assert.Equal(t, tc.expErr, err.Error())
			}
			assert.True(t, reflect.DeepEqual(*tc.config, tc.expConfig))
		})
	}
}

func Test_shardSuites_withSplit(t *testing.T) {
	dir := fs.NewDir(t, "testcafe",
		fs.WithDir("tests",
			fs.WithMode(0755),
			fs.WithDir("dir1",
				fs.WithMode(0755),
				fs.WithFile("example1.tests.js", "", fs.WithMode(0644)),
			),
			fs.WithDir("dir2",
				fs.WithMode(0755),
				fs.WithFile("example2.tests.js", "", fs.WithMode(0644)),
			),
			fs.WithDir("dir3",
				fs.WithMode(0755),
				fs.WithFile("example3.tests.js", "", fs.WithMode(0644)),
			),
		),
	)
	defer dir.Remove()

	// Beginning state
	rootDir := dir.Path()
	origSuites := []Suite{
		{
			Name: "Demo Suite",
			Src:  []string{"tests/**/*.js"},
			Shard: "spec",
		},
	}

	expectedSuites := []Suite{
		{
			Name: "Demo Suite - tests/dir1/example1.tests.js",
			Src:  []string{"tests/dir1/example1.tests.js"},
			Shard: "spec",
		},
		{
			Name: "Demo Suite - tests/dir2/example2.tests.js",
			Src:  []string{"tests/dir2/example2.tests.js"},
			Shard: "spec",
		},
		{
			Name: "Demo Suite - tests/dir3/example3.tests.js",
			Src:  []string{"tests/dir3/example3.tests.js"},
			Shard: "spec",
		},
	}
	var err error
	var suites []Suite

	// Absolute path
	suites, err = shardSuites(rootDir, origSuites)

	assert.Equal(t, err, nil)
	assert.Equal(t, expectedSuites, suites)

	// Relative path
	if err := os.Chdir(rootDir); err != nil {
		t.Errorf("Unexpected error %s", err)
	}
	suites, err = shardSuites(".", origSuites)

	assert.Equal(t, err, nil)
	assert.Equal(t, expectedSuites, suites)
}


func Test_shardSuites_withoutSplit(t *testing.T) {
	origSuites := []Suite{
		{
			Name: "Demo Suite",
			Src:  []string{"tests/**/*.js"},
		},
	}
	var err error
	var suites []Suite

	// Absolute path
	suites, err = shardSuites("", origSuites)

	assert.Equal(t, err, nil)
	assert.Equal(t, origSuites, suites)
}