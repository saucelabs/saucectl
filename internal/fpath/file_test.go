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

func TestList(t *testing.T) {
	/*
		Create the following structure:
			mytestfiles
			mytestfiles/foo.js
			mytestfiles/subdir/bar.js
	*/
	dir := fs.NewDir(t, "mytestfiles",
		fs.WithFile("foo.js", "foo", fs.WithMode(0755)),
		fs.WithDir("mysubdir", fs.WithFile("bar.js", "bar", fs.WithMode(0755))),
	)
	defer dir.Remove()

	type args struct {
		dir     string
		pattern string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name:    "find all .js files",
			args:    args{dir: dir.Path(), pattern: ".*.js"},
			want:    2,
			wantErr: false,
		},
		{
			name:    "find one",
			args:    args{dir: dir.Path(), pattern: "bar.js"},
			want:    1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := List(tt.args.dir, tt.args.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.want {
				t.Errorf("List() got = %v, which is %d in length, but want %d", got, len(got), tt.want)
			}
		})
	}
}

func TestCopy(t *testing.T) {
	dir := fs.NewDir(t, "mytestfiles",
		fs.WithFile("foo.js", "foo", fs.WithMode(0755)),
		fs.WithDir("mysubdir", fs.WithFile("bar.js", "bar", fs.WithMode(0755))),
	)
	defer dir.Remove()

	dest := fs.NewDir(t, "testy")
	defer dest.Remove()

	type args struct {
		src    string
		target string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name:    "nested",
			args:    args{src: dir.Path(), target: dest.Join(dir.Path())},
			want:    2,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeepCopy(tt.args.src, tt.args.target); (err != nil) != tt.wantErr {
				t.Errorf("DeepCopy() error = %v, wantErr %v", err, tt.wantErr)
			}
			ff, err := List(dest.Path(), ".*.js")
			if err != nil {
				t.Errorf("Failed to list contents of directory %v, instead got error = %v", dest.Path(), err)
			}
			if tt.want != len(ff) {
				t.Errorf("Want %d files at destination, but found %d", tt.want, len(ff))
			}
		})
	}
}
