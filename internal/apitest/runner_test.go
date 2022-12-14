package apitest

import (
	"gotest.tools/v3/fs"
	"reflect"
	"testing"
)

func Test_findTests(t *testing.T) {
	dir := fs.NewDir(t, "tests",
		fs.WithDir("tests",
			fs.WithDir("01_basic_test", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
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
