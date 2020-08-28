package fpath

import (
	"gotest.tools/v3/fs"
	"reflect"
	"testing"
)

func TestGlobs(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("some.foo.js", "foo", fs.WithMode(0755)),
		fs.WithFile("some.other.bar.js", "bar", fs.WithMode(0755)))
	defer dir.Remove()

	type args struct {
		patterns []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "find one",
			args: args{[]string{dir.Path() + "/some.foo.js"}},
			want: []string{dir.Join("some.foo.js")},
		},
		{
			name: "find all",
			args: args{[]string{dir.Path() + "/*.js"}},
			want: []string{dir.Join("some.foo.js"), dir.Join("some.other.bar.js")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Globs(tt.args.patterns); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Globs() = %v, want %v", got, tt.want)
			}
		})
	}
}
