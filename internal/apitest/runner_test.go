package apitest

import (
	"gotest.tools/v3/fs"
	"reflect"
	"testing"
)

func Test_findTests(t *testing.T) {
	dir := fs.NewDir(t, "tests",
		fs.WithDir("01 basic test", fs.WithMode(0755),
			fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
			fs.WithFile("input.yaml", "", fs.WithMode(0644)),
		),
		fs.WithDir("02 extended test", fs.WithMode(0755),
			fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
			fs.WithFile("input.yaml", "", fs.WithMode(0644)),
		),
		fs.WithDir("03 no test", fs.WithMode(0755),
			fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
		),
		fs.WithDir("04 no test", fs.WithMode(0755),
			fs.WithFile("input.yaml", "", fs.WithMode(0644)),
		),
		fs.WithDir("05 nested tests", fs.WithMode(0755),
			fs.WithDir("06 nested child01", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
			),
			fs.WithDir("07 nested child02", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
			),
		),
		fs.WithDir("08 hybrid tests", fs.WithMode(0755),
			fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
			fs.WithFile("input.yaml", "", fs.WithMode(0644)),
			fs.WithDir("09 hybrid tests", fs.WithMode(0755),
				fs.WithFile("unit.yaml", "", fs.WithMode(0644)),
				fs.WithFile("input.yaml", "", fs.WithMode(0644)),
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
				"tests/01 basic test",
				"tests/02 extended test",
				"tests/03 no test",
				"tests/04 no test",
				"tests/05 nested tests/06 nested child01",
				"tests/05 nested tests/07 nested child02",
				"tests/08 hybrid tests",
				"tests/08 hybrid tests/09 hybrid tests",
			},
		},
		{
			name: "single filter",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"01 basic test",
				},
			},
			want: []string{"tests/01 basic test"},
		},
		{
			name: "single filter - no match",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"dummy test filter",
				},
			},
			want: []string{"tests/01 basic test"},
		},
		{
			name: "multiple filters",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"01 basic test",
					"02 extended test",
				},
			},
			want: []string{
				"tests/01 basic test",
				"tests/02 extended test",
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
			want: []string{},
		},
		{
			name: "single nested filters",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"05 nested tests",
				},
			},
			want: []string{
				"tests/05 nested tests/06 nested child01",
				"tests/05 nested tests/07 nested child02",
			},
		},
		{
			name: "single nested hybrid filters",
			args: args{
				rootDir: dir.Path(),
				match: []string{
					"08 hybrid tests",
				},
			},
			want: []string{
				"tests/08 hybrid tests",
				"tests/08 hybrid tests/09 hybrid tests",
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
