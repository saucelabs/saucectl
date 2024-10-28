package saucecloud

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/saucecloud/zip"
	"github.com/saucelabs/saucectl/internal/sauceignore"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/fs"
)

func TestSignalDetection(t *testing.T) {
	r := CloudRunner{JobService: JobService{}}
	assert.False(t, r.interrupted)
	c := r.registerSkipSuitesOnSignal()
	defer unregisterSignalCapture(c)

	c <- syscall.SIGINT

	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()

	// Wait for interrupt to be processed, as it happens asynchronously.
	for {
		select {
		case <-deadline.C:
			assert.True(t, r.interrupted)
			return
		default:
			if r.interrupted {
				return
			}
			time.Sleep(1 * time.Nanosecond) // allow context switch
		}
	}
}

func TestRunJobsSkipped(t *testing.T) {
	r := CloudRunner{}
	r.interrupted = true

	opts := make(chan job.StartOptions)
	results := make(chan result)

	go r.runJobs(opts, results)
	opts <- job.StartOptions{}
	close(opts)
	res := <-results
	assert.Nil(t, res.err)
	assert.True(t, res.skipped)
}

func TestCloudRunner_archiveNodeModules(t *testing.T) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "saucectl-app-payload-")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(tempDir)

	projectsDir := fs.NewDir(t, "project",
		fs.WithDir("has-mods",
			fs.WithDir("node_modules",
				fs.WithDir("mod1",
					fs.WithFile("package.json", "{}"),
				),
			),
		),
		fs.WithDir("no-mods"),
		fs.WithDir("empty-mods",
			fs.WithDir("node_modules"),
		),
	)
	defer projectsDir.Remove()

	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get the current working dir: %v", err)
	}

	if err := os.Chdir(projectsDir.Path()); err != nil {
		t.Errorf("Failed to change the current working dir: %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("Failed to change the current working dir back to original: %v", err)
		}
	}()

	type fields struct {
		NPMDependencies []string
	}
	type args struct {
		tempDir string
		rootDir string
		matcher sauceignore.Matcher
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			"want to include mods, but node_modules does not exist",
			fields{
				NPMDependencies: []string{"mod1"},
			},
			args{
				tempDir: tempDir,
				rootDir: "no-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			"",
			func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.EqualError(t, err, "unable to access 'node_modules' folder, but you have npm dependencies defined in your configuration; ensure that the folder exists and is accessible", args)
			},
		},
		{
			"have and want mods, but mods are ignored",
			fields{
				NPMDependencies: []string{"mod1"},
			},
			args{
				tempDir: tempDir,
				rootDir: "has-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{sauceignore.NewPattern("/has-mods/node_modules")}),
			},
			"",
			func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.EqualError(t, err, "'node_modules' is ignored by sauceignore, but you have npm dependencies defined in your project; please remove 'node_modules' from your sauceignore file", args)
			},
		},
		{
			"have mods, don't want them and they are ignored",
			fields{
				NPMDependencies: []string{}, // no mods selected, because we don't want any
			},
			args{
				tempDir: tempDir,
				rootDir: "has-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{sauceignore.NewPattern("/has-mods/node_modules")}),
			},
			"",
			assert.NoError,
		},
		{
			"no mods wanted and no mods exist",
			fields{
				NPMDependencies: []string{},
			},
			args{
				tempDir: tempDir,
				rootDir: "no-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			"",
			assert.NoError,
		},
		{
			"has and wants mods (happy path)",
			fields{
				NPMDependencies: []string{"mod1"},
			},
			args{
				tempDir: tempDir,
				rootDir: "has-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			filepath.Join(tempDir, "node_modules.zip"),
			assert.NoError,
		},
		{
			"want mods, but node_modules folder is empty",
			fields{
				NPMDependencies: []string{"mod1"},
			},
			args{
				tempDir: tempDir,
				rootDir: "empty-mods",
				matcher: sauceignore.NewMatcher([]sauceignore.Pattern{}),
			},
			"",
			func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.EqualError(t, err, "unable to find required dependencies; please check 'node_modules' folder and make sure the dependencies exist", args)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CloudRunner{
				NPMDependencies: tt.fields.NPMDependencies,
			}
			got, err := zip.ArchiveNodeModules(tt.args.tempDir, tt.args.rootDir, tt.args.matcher, r.NPMDependencies)
			if !tt.wantErr(t, err, fmt.Sprintf("archiveNodeModules(%v, %v, %v)", tt.args.tempDir, tt.args.rootDir, tt.args.matcher)) {
				return
			}
			assert.Equalf(t, tt.want, got, "archiveNodeModules(%v, %v, %v)", tt.args.tempDir, tt.args.rootDir, tt.args.matcher)
		})
	}
}

func Test_arrayContains(t *testing.T) {
	type args struct {
		list []string
		want string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Empty set",
			args: args{
				list: []string{},
				want: "value",
			},
			want: false,
		},
		{
			name: "Complete set - false",
			args: args{
				list: []string{"val1", "val2", "val3"},
				want: "value",
			},
			want: false,
		},
		{
			name: "Found",
			args: args{
				list: []string{"val1", "val2", "val3"},
				want: "val1",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, arrayContains(tt.args.list, tt.args.want), "arrayContains(%v, %v)", tt.args.list, tt.args.want)
		})
	}
}
