package ini

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"gotest.tools/v3/fs"

	"github.com/stretchr/testify/assert"
)

func Test_frameworkExtValidator(t *testing.T) {
	dir := fs.NewDir(t, "apps",
		fs.WithFile("my.zip", "--", fs.WithMode(0644)),
		fs.WithFile("my.json", "--", fs.WithMode(0644)),
		fs.WithFile("my.js", "--", fs.WithMode(0644)),
		fs.WithFile("my.apk", "--", fs.WithMode(0644)),
		fs.WithFile("my.ipa", "--", fs.WithMode(0644)),
		fs.WithDir("my.app", fs.WithMode(0755)),
	)
	defer dir.Remove()

	type args struct {
		framework        string
		frameworkVersion string
		filename         string
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "espresso - apk",
			args: args{
				framework: "espresso",
				filename:  dir.Join("my.apk"),
			},
			want: nil,
		},
		{
			name: "espresso - .zip",
			args: args{
				framework: "espresso",
				filename:  dir.Join("my.zip"),
			},
			want: errors.New("invalid extension. must be one of the following: .apk, .aab"),
		},
		{
			name: "xcuitest - .ipa",
			args: args{
				framework: "xcuitest",
				filename:  dir.Join("my.ipa"),
			},
			want: nil,
		},
		{
			name: "xcuitest - .app",
			args: args{
				framework: "xcuitest",
				filename:  dir.Join("my.app"),
			},
			want: nil,
		},
		{
			name: "xcuitest - .zip",
			args: args{
				framework: "xcuitest",
				filename:  dir.Join("my.zip"),
			},
			want: errors.New("invalid extension. must be one of the following: .ipa, .app"),
		},
		{
			name: "cypress - .json",
			args: args{
				framework:        "cypress",
				frameworkVersion: "9.7.0",
				filename:         dir.Join("my.json"),
			},
			want: nil,
		},
		{
			name: "cypress 10 - .js",
			args: args{
				framework:        "cypress",
				frameworkVersion: "10.3.1",
				filename:         dir.Join("my.js"),
			},
			want: nil,
		},
		{
			name: "cypress - .zip",
			args: args{
				framework:        "cypress",
				frameworkVersion: "9.7.0",
				filename:         dir.Join("my.zip"),
			},
			want: errors.New("invalid extension. must be one of the following: .json"),
		},
		{
			name: "cypress 10 - .zip",
			args: args{
				framework:        "cypress",
				frameworkVersion: "10.3.1",
				filename:         dir.Join("my.zip"),
			},
			want: errors.New("invalid extension. must be one of the following: .js, .ts, .mjs, .cjs"),
		},
		{
			name: "espresso - bad .apk",
			args: args{
				framework: "espresso",
				filename:  "bad.apk",
			},
			want: errors.New("bad.apk: stat bad.apk: no such file or directory"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (frameworkExtValidator(tt.args.framework, tt.args.frameworkVersion))(tt.args.filename); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extValidator() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_saveSauceIgnore(t *testing.T) {
	dir := fs.NewDir(t, "tests",
		fs.WithDir("open", fs.WithMode(0755)),
		fs.WithDir("closed", fs.WithMode(0100)),
	)
	defer dir.Remove()

	type args struct {
		content  string
		location string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Working",
			args: args{
				content:  "demo-content",
				location: dir.Join("open"),
			},
			wantErr: false,
		},
		// Test Disabled as it's working on linux.
		//{
		//	name: "Access denied",
		//	args: args{
		//		content:  "demo-content",
		//		location: dir.Join("closed"),
		//	},
		//	wantErr: true,
		//},
	}
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("os.Getwd() failed: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err = os.Chdir(tt.args.location); err != nil {
				t.Errorf("os.Chdir() failed: %v", err)
			}
			defer func() {
				if err = os.Chdir(pwd); err != nil {
					t.Errorf("os.Chdir() failed: %v", err)
				}
			}()

			err = saveSauceIgnore(tt.args.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("saveSauceIgnore() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			// Read content
			content, err := os.ReadFile(".sauceignore")
			if err != nil {
				t.Errorf("unable to open .sauceignore created file: %v", err)
			}
			if string(content) != tt.args.content {
				t.Errorf("saveSauceIgnore() content mismatch, got: %v, want: %v", string(content), tt.args.content)
			}
		})
	}
}

func Test_saveSauceConfig(t *testing.T) {
	dir := fs.NewDir(t, "tests",
		fs.WithDir("non-existing-dir", fs.WithMode(0755)),
		fs.WithDir("existing-dir", fs.WithMode(0755),
			fs.WithDir(".sauce", fs.WithMode(0755))),
		fs.WithDir("existing-file", fs.WithMode(0755),
			fs.WithDir(".sauce", fs.WithMode(0755),
				fs.WithFile("config.yml", "dummy-file", fs.WithMode(0644)))),
		fs.WithDir("existing-file-denied", fs.WithMode(0755),
			fs.WithDir(".sauce", fs.WithMode(0755),
				fs.WithFile("config.yml", "dummy-file", fs.WithMode(0400)))),
		fs.WithDir("existing-dir-denied", fs.WithMode(0755),
			fs.WithDir(".sauce", fs.WithMode(0100))),
		fs.WithDir("denied", fs.WithMode(0100)),
	)
	defer dir.Remove()

	type args struct {
		content  interface{}
		location string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "non-existing .sauce dir",
			args: args{
				content:  map[string]string{"key": "value", "key2": "value2"},
				location: dir.Join("non-existing-dir"),
			},
			want:    "key: value\nkey2: value2\n",
			wantErr: false,
		},
		{
			name: "existing .sauce dir",
			args: args{
				content:  map[string]string{"key": "value", "key2": "value2"},
				location: dir.Join("existing-dir"),
			},
			want:    "key: value\nkey2: value2\n",
			wantErr: false,
		},
		{
			name: "existing .sauce/config.yml file",
			args: args{
				content:  map[string]string{"key": "value", "key2": "value2"},
				location: dir.Join("existing-file"),
			},
			want:    "key: value\nkey2: value2\n",
			wantErr: false,
		},
		// Those tests are for now disabled as they are failing on linux only.
		//
		//{
		//	name: "existing .sauce/config.yml file - access denied",
		//	args: args{
		//		content:  map[string]string{"key": "value", "key2": "value2"},
		//		location: dir.Join("existing-file-denied"),
		//	},
		//	want:    ``,
		//	wantErr: true,
		//},
		//{
		//	name: "existing .sauce dir - access denied",
		//	args: args{
		//		content:  map[string]string{"key": "value", "key2": "value2"},
		//		location: dir.Join("existing-dir-denied"),
		//	},
		//	want:    ``,
		//	wantErr: true,
		//},
		//{
		//	name: "empty dir - access denied",
		//	args: args{
		//		content:  map[string]string{"key": "value", "key2": "value2"},
		//		location: dir.Join("denied"),
		//	},
		//	want:    ``,
		//	wantErr: true,
		//},
	}
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("os.Getwd() failed: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err = os.Chdir(tt.args.location); err != nil {
				t.Errorf("os.Chdir() failed: %v", err)
			}
			defer func() {
				if err = os.Chdir(pwd); err != nil {
					t.Errorf("os.Chdir() failed: %v", err)
				}
			}()

			err = saveSauceConfig(tt.args.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("saveSauceConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			// Read content
			content, err := os.ReadFile(".sauce/config.yml")
			if err != nil {
				t.Errorf("unable to open .sauceignore created file: %v", err)
			}
			if string(content) != tt.want {
				t.Errorf("saveSauceConfig() content mismatch, got: %v, want: %v", string(content), tt.want)
			}
		})
	}
}

func Test_saveConfigurationFiles(t *testing.T) {
	dir := fs.NewDir(t, "workdir")
	defer dir.Remove()

	pwd, _ := os.Getwd()
	if err := os.Chdir(dir.Path()); err != nil {
		t.Errorf("failed to change directory to %s: %v", dir.Path(), err)
	}
	defer func() {
		if err := os.Chdir(pwd); err != nil {
			t.Errorf("failed to change directory to %s: %v", pwd, err)
		}
	}()

	calledConfig := false

	oldCypressConfig := configurators["cypress"]
	oldXcuitestConfig := configurators["xcuitest"]
	configurators["cypress"] = func(*initConfig) interface{} {
		calledConfig = true
		return map[string]string{}
	}
	configurators["xcuitest"] = func(*initConfig) interface{} {
		calledConfig = true
		return map[string]string{}
	}
	defer func() {
		configurators["cypress"] = oldCypressConfig
		configurators["xcuitest"] = oldXcuitestConfig
	}()

	tests := []struct {
		name         string
		framework    string
		want         []string
		calledConfig bool
		wantErr      bool
	}{
		{
			name:         "Cypress - config.yml+.sauceignore",
			framework:    "cypress",
			want:         []string{".sauce/config.yml", ".sauceignore"},
			calledConfig: true,
		},
		{
			name:         "XCUITest - config.yml",
			framework:    "xcuitest",
			want:         []string{".sauce/config.yml"},
			calledConfig: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledConfig = false
			got, err := saveConfigurationFiles(&initConfig{
				frameworkName: tt.framework,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("saveConfigurationFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("saveConfigurationFiles() got = %v, want %v", got, tt.want)
			}
			if calledConfig != tt.calledConfig {
				t.Errorf("saveConfigurationFiles() calledConfig: got = %v, want %v", calledConfig, tt.calledConfig)
			}
		})
	}
}

func Test_completeBasic(t *testing.T) {
	dir := fs.NewDir(t, "tests",
		fs.WithFile("agavetoiledNostalgicrecommend.txt", ""),
		fs.WithFile("clockerZealousResistor.txt", ""),
		fs.WithFile("crouchedaurochsAbrasiveWidow.txt", ""),
		fs.WithFile("gougeDeportpostscriptOverhangs.txt", ""),
		fs.WithFile("hairyAxlespropsRoomer.txt", ""),
		fs.WithFile("mismatchedCampaignspeckplatonism.txt", ""),
		fs.WithFile("nineteenenviableToddDisjunct.txt", ""),
		fs.WithFile("suspendedsimmonsbuckboardsdrum.txt", ""),
	)
	defer dir.Remove()

	pwd, _ := os.Getwd()
	if err := os.Chdir(dir.Path()); err != nil {
		t.Errorf("failed to change directory to %s: %v", dir.Path(), err)
	}
	defer func() {
		if err := os.Chdir(pwd); err != nil {
			t.Errorf("failed to change directory to %s: %v", pwd, err)
		}
	}()

	type args struct {
		toComplete string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Empty",
			args: args{
				toComplete: "",
			},
			want: []string{
				"agavetoiledNostalgicrecommend.txt",
				"clockerZealousResistor.txt",
				"crouchedaurochsAbrasiveWidow.txt",
				"gougeDeportpostscriptOverhangs.txt",
				"hairyAxlespropsRoomer.txt",
				"mismatchedCampaignspeckplatonism.txt",
				"nineteenenviableToddDisjunct.txt",
				"suspendedsimmonsbuckboardsdrum.txt",
			},
		},
		{
			name: "complete \"c\"",
			args: args{
				toComplete: "c",
			},
			want: []string{"clockerZealousResistor.txt", "crouchedaurochsAbrasiveWidow.txt"},
		},
		{
			name: "complete \"h\"",
			args: args{
				toComplete: "h",
			},
			want: []string{"hairyAxlespropsRoomer.txt"},
		},
		{
			name: "complete \"z\"",
			args: args{
				toComplete: "z",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := completeBasic(tt.args.toComplete); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("completeBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sortVersions(t *testing.T) {
	type args struct {
		versions []string
	}
	tests := []struct {
		name     string
		args     args
		expected []string
	}{
		{
			name: "Already sorted",
			args: args{
				versions: []string{"11.0", "10.0", "9.0", "8.1", "8.0", "7.0"},
			},
			expected: []string{"11.0", "10.0", "9.0", "8.1", "8.0", "7.0"},
		},
		{
			name: "Reversed",
			args: args{
				versions: []string{"7.0", "8.0", "8.1", "9.0", "10.0", "11.0"},
			},
			expected: []string{"11.0", "10.0", "9.0", "8.1", "8.0", "7.0"},
		},
		{
			name: "Random",
			args: args{
				versions: []string{"9.0", "8.1", "7.0", "10.0", "11.0", "8.0"},
			},
			expected: []string{"11.0", "10.0", "9.0", "8.1", "8.0", "7.0"},
		},
		{
			name: "Incomplete values",
			args: args{
				versions: []string{"9", "8.1", "7", "10.0.1", "11.0", "8.0"},
			},
			expected: []string{"11.0", "10.0.1", "9", "8.1", "8.0", "7"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortVersions(tt.args.versions)
			assert.Equal(t, tt.expected, tt.args.versions)
		})
	}
}

func TestCommon_getMajorVersion(t *testing.T) {
	testCases := []struct {
		name    string
		version string
		expRes  int
	}{
		{
			name:    "get valid major version",
			version: "10.3.1",
			expRes:  10,
		},
		{
			name:   "version is empty",
			expRes: 0,
		},
		{
			name:    "version is invalid",
			version: "test,",
			expRes:  0,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getMajorVersion(tc.version)
			assert.Equal(t, tc.expRes, result)
		})
	}
}

func Test_dockerImageValidator(t *testing.T) {
	tests := []struct {
		image string
		want  error
	}{
		{
			image: "alpine",
			want:  nil,
		},
		{
			image: "alpine:latest",
			want:  nil,
		},
		{
			image: "_/alpine",
			want:  nil,
		},
		{
			image: "_/alpine:latest",
			want:  nil,
		},
		{
			image: "alpine:3.7",
			want:  nil,
		},
		{
			image: "docker.example.com/gmr/alpine:3.7",
			want:  nil,
		},
		{
			image: "docker.example.com:5000/gmr/alpine:latest",
			want:  nil,
		},
		{
			image: "pse/anabroker:latest",
			want:  nil,
		},
		{
			image: "buggy::latest",
			want:  errors.New("buggy::latest is not a valid docker image"),
		},
		{
			image: "buggy:",
			want:  errors.New("buggy: is not a valid docker image"),
		},
	}
	val := dockerImageValidator()
	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			assert.Equalf(t, tt.want, val(tt.image), "dockerImageValidator()")
		})
	}
}
