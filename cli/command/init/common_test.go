package init

import (
	"errors"
	"gotest.tools/v3/fs"
	"os"
	"reflect"
	"testing"
)

func Test_extValidator(t *testing.T) {
	dir := fs.NewDir(t, "apps",
		fs.WithFile("my.zip", "--", fs.WithMode(0644)),
		fs.WithFile("my.json", "--", fs.WithMode(0644)),
		fs.WithFile("my.apk", "--", fs.WithMode(0644)),
		fs.WithFile("my.ipa", "--", fs.WithMode(0644)),
		fs.WithDir("my.app", fs.WithMode(0755)),
	)
	defer dir.Remove()

	type args struct {
		framework string
		filename  string
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
			want: errors.New("invalid extension. must be one of the following: .apk"),
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
				framework: "cypress",
				filename:  dir.Join("my.json"),
			},
			want: nil,
		},
		{
			name: "cypress - .zip",
			args: args{
				framework: "cypress",
				filename:  dir.Join("my.zip"),
			},
			want: errors.New("invalid extension. must be one of the following: .json"),
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
			if got := (extValidator(tt.args.framework))(tt.args.filename); !reflect.DeepEqual(got, tt.want) {
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
		{
			name: "Access denied",
			args: args{
				content:  "demo-content",
				location: dir.Join("closed"),
			},
			wantErr: true,
		},
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
		{
			name: "existing .sauce/config.yml file - access denied",
			args: args{
				content:  map[string]string{"key": "value", "key2": "value2"},
				location: dir.Join("existing-file-denied"),
			},
			want:    ``,
			wantErr: true,
		},
		{
			name: "existing .sauce dir - access denied",
			args: args{
				content:  map[string]string{"key": "value", "key2": "value2"},
				location: dir.Join("existing-dir-denied"),
			},
			want:    ``,
			wantErr: true,
		},
		{
			name: "empty dir - access denied",
			args: args{
				content:  map[string]string{"key": "value", "key2": "value2"},
				location: dir.Join("denied"),
			},
			want:    ``,
			wantErr: true,
		},
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

func Test_uniqSorted(t *testing.T) {
	type args struct {
		ss []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Sorted and unique",
			args: args{
				ss: []string{"aaa", "bbb", "ccc"},
			},
			want: []string{"aaa", "bbb", "ccc"},
		},
		{
			name: "Not sorted and unique",
			args: args{
				ss: []string{"ccc", "aaa", "bbb"},
			},
			want: []string{"aaa", "bbb", "ccc"},
		},
		{
			name: "Not sorted and not unique",
			args: args{
				ss: []string{"ccc", "bbb", "aaa", "bbb", "aaa", "ccc"},
			},
			want: []string{"aaa", "bbb", "ccc"},
		},
		{
			name: "Sorted and not unique",
			args: args{
				ss: []string{"aaa", "aaa", "bbb", "bbb", "ccc"},
			},
			want: []string{"aaa", "bbb", "ccc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniqSorted(tt.args.ss); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("uniqSorted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_saveConfigurationFiles(t *testing.T) {
	dir := fs.NewDir(t, "workdir")
	defer dir.Remove()

	pwd, _ := os.Getwd()
	os.Chdir(dir.Path())
	defer os.Chdir(pwd)

	calledConfig := false
	calledSauceignore := false

	oldCypressConfig := configurators["cypress"]
	oldXcuitestConfig := configurators["xcuitest"]
	configurators["cypress"] = func(cfg *initConfig) interface{} {
		calledConfig = true
		return map[string]string{}
	}
	configurators["xcuitest"] = func(cfg *initConfig) interface{} {
		calledConfig = true
		return map[string]string{}
	}
	oldCypressSauceignore := sauceignoreGenerators["cypress"]
	sauceignoreGenerators["cypress"] = func() string {
		calledSauceignore = true
		return ""
	}
	defer func() {
		configurators["cypress"] = oldCypressConfig
		configurators["xcuitest"] = oldXcuitestConfig
		sauceignoreGenerators["cypress"] = oldCypressSauceignore
	}()

	tests := []struct {
		name         string
		framework    string
		want         []string
		calledConfig bool
		calledIgnore bool
		wantErr      bool
	}{
		{
			name:         "Cypress - config.yml+.sauceignore",
			framework:    "cypress",
			want:         []string{".sauce/config.yml", ".sauceignore"},
			calledConfig: true,
			calledIgnore: true,
		},
		{
			name:         "XCUITest - config.yml",
			framework:    "xcuitest",
			want:         []string{".sauce/config.yml"},
			calledConfig: true,
			calledIgnore: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledConfig = false
			calledSauceignore = false
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
			if calledSauceignore != tt.calledIgnore {
				t.Errorf("saveConfigurationFiles() calledSauceignore: got = %v, want %v", calledSauceignore, tt.calledIgnore)
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
	os.Chdir(dir.Path())
	defer os.Chdir(pwd)

	type args struct {
		folder     string
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
