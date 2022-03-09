package cypress

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

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

func Test_loadCypressConfiguration(t *testing.T) {
	tests := []struct {
		creator     func(t *testing.T) *fs.Dir
		name        string
		errFileName string
		wantErr     string
	}{
		{
			name: "Valid empty config",
			creator: func(t *testing.T) *fs.Dir {
				dir := fs.NewDir(t, "test-case", fs.WithMode(0755),
					fs.WithFile("cypress.json", "{}", fs.WithMode(0644)),
					fs.WithDir("cypress", fs.WithMode(0755),
						fs.WithDir("integration", fs.WithMode(0755))))
				return dir
			},
			wantErr: "",
		},
		{
			name: "Valid config with custom integration folder",
			creator: func(t *testing.T) *fs.Dir {
				dir := fs.NewDir(t, "test-case", fs.WithMode(0755),
					fs.WithFile("cypress.json", `{"integrationFolder":"e2e/integration"}`, fs.WithMode(0644)),
					fs.WithDir("e2e", fs.WithMode(0755),
						fs.WithDir("integration", fs.WithMode(0755))))
				return dir
			},
			wantErr: "",
		},
		{
			name: "Valid config with custom integration/fixtures folders, plugins/support files",
			creator: func(t *testing.T) *fs.Dir {
				dir := fs.NewDir(t, "test-case", fs.WithMode(0755),
					fs.WithFile("cypress.json", `{"integrationFolder":"e2e/integration", "fixturesFolder": "e2e/fixtures", "pluginsFile": "e2e/plugins-custom/index.js", "supportFile": "e2e/support-custom/index.js"}`, fs.WithMode(0644)),
					fs.WithDir("e2e", fs.WithMode(0755),
						fs.WithDir("integration", fs.WithMode(0755)),
						fs.WithDir("fixtures", fs.WithMode(0755)),
						fs.WithDir("plugins-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644))),
						fs.WithDir("support-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644)))))
				return dir
			},
			wantErr: "",
		},
		{
			name: "Invalid file",
			creator: func(t *testing.T) *fs.Dir {
				dir := fs.NewDir(t, "test-case", fs.WithMode(0755),
					fs.WithFile("cypress.json", `{"integrationFo}`, fs.WithMode(0644)))
				return dir
			},
			wantErr: "unexpected EOF",
		},
		{
			name: "Un-existing integrationFolder",
			creator: func(t *testing.T) *fs.Dir {
				dir := fs.NewDir(t, "test-case", fs.WithMode(0755),
					fs.WithFile("cypress.json", `{"integrationFolder":"e2e/integration-absent", "fixturesFolder": "e2e/fixtures", "pluginsFile": "e2e/plugins-custom/index.js", "supportFile": "e2e/support-custom/index.js"}`, fs.WithMode(0644)),
					fs.WithDir("e2e", fs.WithMode(0755),
						fs.WithDir("integration", fs.WithMode(0755)),
						fs.WithDir("fixtures", fs.WithMode(0755)),
						fs.WithDir("plugins-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644))),
						fs.WithDir("support-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644)))))
				return dir
			},
			errFileName: "e2e/integration-absent",
			wantErr:     "stat %s: no such file or directory",
		},
		{
			name: "Un-existing fixture folder",
			creator: func(t *testing.T) *fs.Dir {
				dir := fs.NewDir(t, "test-case", fs.WithMode(0755),
					fs.WithFile("cypress.json", `{"integrationFolder":"e2e/integration", "fixturesFolder": "e2e/fixtures-absent", "pluginsFile": "e2e/plugins-custom/index.js", "supportFile": "e2e/support-custom/index.js"}`, fs.WithMode(0644)),
					fs.WithDir("e2e", fs.WithMode(0755),
						fs.WithDir("integration", fs.WithMode(0755)),
						fs.WithDir("fixtures", fs.WithMode(0755)),
						fs.WithDir("plugins-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644))),
						fs.WithDir("support-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644)))))
				return dir
			},
			errFileName: "e2e/fixtures-absent",
			wantErr:     "stat %s: no such file or directory",
		},
		{
			name: "Un-existing plugins file",
			creator: func(t *testing.T) *fs.Dir {
				dir := fs.NewDir(t, "test-case", fs.WithMode(0755),
					fs.WithFile("cypress.json", `{"integrationFolder":"e2e/integration", "fixturesFolder": "e2e/fixtures", "pluginsFile": "e2e/plugins-custom/index-fake.js", "supportFile": "e2e/support-custom/index.js"}`, fs.WithMode(0644)),
					fs.WithDir("e2e", fs.WithMode(0755),
						fs.WithDir("integration", fs.WithMode(0755)),
						fs.WithDir("fixtures", fs.WithMode(0755)),
						fs.WithDir("plugins-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644))),
						fs.WithDir("support-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644)))))
				return dir
			},
			errFileName: "e2e/plugins-custom/index-fake.js",
			wantErr:     "stat %s: no such file or directory",
		},
		{
			name: "Un-existing support file",
			creator: func(t *testing.T) *fs.Dir {
				dir := fs.NewDir(t, "test-case", fs.WithMode(0755),
					fs.WithFile("cypress.json", `{"integrationFolder":"e2e/integration", "fixturesFolder": "e2e/fixtures", "pluginsFile": "e2e/plugins-custom/index.js", "supportFile": "e2e/support-custom/index-fake.js"}`, fs.WithMode(0644)),
					fs.WithDir("e2e", fs.WithMode(0755),
						fs.WithDir("integration", fs.WithMode(0755)),
						fs.WithDir("fixtures", fs.WithMode(0755)),
						fs.WithDir("plugins-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644))),
						fs.WithDir("support-custom", fs.WithMode(0755),
							fs.WithFile("index.js", "", fs.WithMode(0644)))))
				return dir
			},
			errFileName: "e2e/support-custom/index-fake.js",
			wantErr:     "stat %s: no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.creator(t)
			defer d.Remove()
			_, err := loadCypressConfiguration(d.Path(), "cypress.json", ".sauceignore")

			if tt.wantErr != "" {
				expectedErr := tt.wantErr
				if tt.errFileName != "" {
					expectedErr = fmt.Sprintf(tt.wantErr, d.Join(tt.errFileName))
				}
				assert.EqualError(t, err, expectedErr, "ValidateCypressConfiguration() error = %v, wantErr %v", err, expectedErr)
			} else {
				assert.Nil(t, err, "ValidateCypressConfiguration() error = %v, wanted no-error", err)
			}
		})
	}
}

func Test_shardSuites(t *testing.T) {
	dir := fs.NewDir(t, "cypress",
		fs.WithMode(0755),
		fs.WithDir("cypress",
			fs.WithMode(0755),
			fs.WithDir("integration",
				fs.WithMode(0755),
				fs.WithDir("a",
					fs.WithMode(0755),
					fs.WithFile("todo.spec.js", "dummy", fs.WithMode(0644)),
					fs.WithDir("b",
						fs.WithMode(0755),
						fs.WithFile("todo.spec.js", "dummy", fs.WithMode(0644)),
						fs.WithDir("c",
							fs.WithMode(0755),
							fs.WithFile("todo.spec.js", "dummy", fs.WithMode(0644)),
						),
					),
				),
				fs.WithFile("file1.spec.js", "dummy", fs.WithMode(0644)),
				fs.WithFile("file2.spec.js", "dummy", fs.WithMode(0644)),
			)))

	defer dir.Remove()

	oldPwd, _ := os.Getwd()
	defer os.Chdir(oldPwd)
	os.Chdir(dir.Path())

	type args struct {
		cfg    Config
		suites []Suite
	}
	tests := []struct {
		name    string
		args    args
		want    []Suite
		wantErr error
	}{
		{
			name: "Single suite",
			args: args{
				cfg: Config{
					Path:              ".",
					IntegrationFolder: "cypress/integration/",
				},
				suites: []Suite{
					{
						Name: "Demo #1",
						Config: SuiteConfig{
							TestFiles: []string{"**/*.spec.js"},
						},
						Shard: "none",
					},
				},
			},
			want: []Suite{
				{
					Name: "Demo #1",
					Config: SuiteConfig{
						TestFiles: []string{"**/*.spec.js"},
					},
					Shard: "none",
				},
			},
			wantErr: nil,
		},
		{
			name: "Sharded suite",
			args: args{
				cfg: Config{
					Path:              ".",
					IntegrationFolder: "cypress/integration/",
				},
				suites: []Suite{
					{
						Name: "Demo #1",
						Config: SuiteConfig{
							TestFiles: []string{"**/*.spec.js"},
						},
						Shard: "spec",
					},
				},
			},
			want: []Suite{
				{
					Name: "Demo #1 - a/b/c/todo.spec.js",
					Config: SuiteConfig{
						TestFiles: []string{"a/b/c/todo.spec.js"},
					},
					Shard: "spec",
				},
				{
					Name: "Demo #1 - a/b/todo.spec.js",
					Config: SuiteConfig{
						TestFiles: []string{"a/b/todo.spec.js"},
					},
					Shard: "spec",
				},
				{
					Name: "Demo #1 - a/todo.spec.js",
					Config: SuiteConfig{
						TestFiles: []string{"a/todo.spec.js"},
					},
					Shard: "spec",
				},
				{
					Name: "Demo #1 - file1.spec.js",
					Config: SuiteConfig{
						TestFiles: []string{"file1.spec.js"},
					},
					Shard: "spec",
				},
				{
					Name: "Demo #1 - file2.spec.js",
					Config: SuiteConfig{
						TestFiles: []string{"file2.spec.js"},
					},
					Shard: "spec",
				},
			},
			wantErr: nil,
		},
		{
			name: "Sharded suite - no match",
			args: args{
				cfg: Config{
					Path:              ".",
					IntegrationFolder: "cypress/integration/",
				},
				suites: []Suite{
					{
						Name: "Demo #1",
						Config: SuiteConfig{
							TestFiles: []string{"**/*.fail.js"},
						},
						Shard: "spec",
					},
				},
			},
			want:    []Suite{},
			wantErr: errors.New("suite 'Demo #1' patterns have no matching files"),
		},
		{
			name: "Sharded with hard prefix",
			args: args{
				cfg: Config{
					Path:              ".",
					IntegrationFolder: "cypress/integration/",
				},
				suites: []Suite{
					{
						Name: "Demo #1",
						Config: SuiteConfig{
							TestFiles: []string{"a/**/todo.spec.js"},
						},
						Shard: "spec",
					},
				},
			},
			want: []Suite{
				{
					Name: "Demo #1 - a/b/c/todo.spec.js",
					Config: SuiteConfig{
						TestFiles: []string{"a/b/c/todo.spec.js"},
					},
					Shard: "spec",
				},
				{
					Name: "Demo #1 - a/b/todo.spec.js",
					Config: SuiteConfig{
						TestFiles: []string{"a/b/todo.spec.js"},
					},
					Shard: "spec",
				},
				{
					Name: "Demo #1 - a/todo.spec.js",
					Config: SuiteConfig{
						TestFiles: []string{"a/todo.spec.js"},
					},
					Shard: "spec",
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shardSuites(tt.args.cfg, tt.args.suites)
			assert.Equal(t, tt.wantErr, err, "err for shardSuites(%v, %v)", tt.args.cfg, tt.args.suites)
			assert.Equalf(t, tt.want, got, "shardSuites(%v, %v)", tt.args.cfg, tt.args.suites)
		})
	}
}
