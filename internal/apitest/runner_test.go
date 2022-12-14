package apitest

import (
	"github.com/saucelabs/saucectl/internal/apitesting"
	"gotest.tools/v3/fs"
	"path"
	"reflect"
	"testing"
)

func createTestDirs(t *testing.T) *fs.Dir {
	return fs.NewDir(t, "tests",
		fs.WithDir("tests",
			fs.WithDir("01_basic_test", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "yaml-unit-content", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "yaml-input-content", fs.WithMode(0644)),
			),
			fs.WithDir("02_extended_test", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
			),
			fs.WithDir("03_no_test", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
			),
			fs.WithDir("04_no_test", fs.WithMode(0755),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
			),
			fs.WithDir("05_nested_tests", fs.WithMode(0755),
				fs.WithDir("06_nested_child01", fs.WithMode(0755),
					fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
					fs.WithFile("input.yaml", "", fs.WithMode(0644)),
				),
				fs.WithDir("07_nested_child02", fs.WithMode(0755),
					fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
					fs.WithFile("input.yaml", "", fs.WithMode(0644)),
				),
			),
			fs.WithDir("08_hybrid_tests", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
				fs.WithDir("09_hybrid_tests", fs.WithMode(0755),
					fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
					fs.WithFile("input.yaml", "", fs.WithMode(0644)),
				),
			),
		),
	)
}

func Test_findTests(t *testing.T) {
	dir := createTestDirs(t)
	defer dir.Remove()

	type args struct {
		rootDir string
		match   []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "no filters",
			args: args{
				rootDir: dir.Path(),
				match:   []string{},
			},
			want: []string{
				"tests/01_basic_test",
				"tests/02_extended_test",
				"tests/05_nested_tests/06_nested_child01",
				"tests/05_nested_tests/07_nested_child02",
				"tests/08_hybrid_tests",
				"tests/08_hybrid_tests/09_hybrid_tests",
			},
		},
		{
			name: "single filter",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"01_basic_test",
				},
			},
			want: []string{"tests/01_basic_test"},
		},
		{
			name: "single filter - no match",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"dummy test filter",
				},
			},
		},
		{
			name: "multiple filters",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"01_basic_test",
					"02_extended_test",
				},
			},
			want: []string{
				"tests/01_basic_test",
				"tests/02_extended_test",
			},
		},
		{
			name: "multiple filters - no match",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"dummy filter",
					"second dummy filter",
				},
			},
		},
		{
			name: "single nested filters",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"05_nested_tests",
				},
			},
			want: []string{
				"tests/05_nested_tests/06_nested_child01",
				"tests/05_nested_tests/07_nested_child02",
			},
		},
		{
			name: "single nested hybrid filters",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"08_hybrid_tests",
				},
			},
			want: []string{
				"tests/08_hybrid_tests",
				"tests/08_hybrid_tests/09_hybrid_tests",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findTests(tt.args.rootDir, tt.args.match); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findTests() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_loadTest(t *testing.T) {
	dir := createTestDirs(t)
	defer dir.Remove()

	type args struct {
		unitPath  string
		inputPath string
		suiteName string
		testName  string
		tags      []string
	}
	tests := []struct {
		name    string
		args    args
		want    apitesting.TestRequest
		wantErr bool
	}{
		{
			name: "Valid loading",
			args: args{
				tags:      []string{},
				testName:  "tests/01_basic_test",
				suiteName: "suiteName",
				inputPath: path.Join(dir.Path(), "tests/01_basic_test/input.yaml"),
				unitPath:  path.Join(dir.Path(), "tests/01_basic_test/unit.yaml"),
			},
			wantErr: false,
			want: apitesting.TestRequest{
				Name:  "suiteName - tests/01_basic_test",
				Tags:  []string{},
				Unit:  "yaml-unit-content",
				Input: "yaml-input-content",
			},
		},
		{
			name: "Buggy loading - missing input",
			args: args{
				tags:      []string{},
				testName:  "tests/03_no_test",
				suiteName: "suiteName",
				inputPath: path.Join(dir.Path(), "tests/03_no_test/input.yaml"),
				unitPath:  path.Join(dir.Path(), "tests/03_no_test/unit.yaml"),
			},
			wantErr: true,
		},
		{
			name: "Buggy loading - missing unit",
			args: args{
				tags:      []string{},
				testName:  "tests/04_no_test",
				suiteName: "suiteName",
				inputPath: path.Join(dir.Path(), "tests/04_no_test/input.yaml"),
				unitPath:  path.Join(dir.Path(), "tests/04_no_test/unit.yaml"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadTest(tt.args.unitPath, tt.args.inputPath, tt.args.suiteName, tt.args.testName, tt.args.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadTest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadTest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildTestName(t *testing.T) {
	type args struct {
		project apitesting.Project
		test    apitesting.Test
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Basic Test",
			args: args{
				test: apitesting.Test{
					Name: "defaultName",
				},
				project: apitesting.Project{
					Name: "projectName",
				},
			},
			want: "projectName - defaultName",
		},
		{
			name: "Only ProjectName",
			args: args{
				project: apitesting.Project{
					Name: "projectName",
				},
			},
			want: "projectName",
		},
		{
			name: "Only TestName",
			args: args{
				test: apitesting.Test{
					Name: "defaultName",
				},
			},
			want: " - defaultName",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildTestName(tt.args.project, tt.args.test); got != tt.want {
				t.Errorf("buildTestName() = %v, want %v", got, tt.want)
			}
		})
	}
}
