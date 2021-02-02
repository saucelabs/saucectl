package fleet

import (
	"context"
	"github.com/saucelabs/saucectl/internal/config"
	"gotest.tools/v3/fs"
	"strings"
	"testing"
)

type testSequencer struct {
	RegisterFunc       func(ctx context.Context, buildID string, testSuites []TestSuite) (string, error)
	NextAssignmentFunc func(ctx context.Context, fleetID, suiteName string) (string, error)
}

func (s *testSequencer) Register(ctx context.Context, buildID string, testSuites []TestSuite) (string, error) {
	return s.RegisterFunc(ctx, buildID, testSuites)
}

func (s *testSequencer) NextAssignment(ctx context.Context, fleetID, suiteName string) (string, error) {
	return s.NextAssignmentFunc(ctx, fleetID, suiteName)
}

func TestRegister(t *testing.T) {
	dir := fs.NewDir(t, "mytestfiles",
		fs.WithFile("foo.js", "foo", fs.WithMode(0755)),
	)
	defer dir.Remove()

	type args struct {
		ctx       context.Context
		seq       Sequencer
		testFiles []string
		suites    []config.Suite
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "simple",
			args: args{
				ctx: context.Background(),
				seq: &testSequencer{RegisterFunc: func(ctx context.Context, buildID string, testSuites []TestSuite) (string, error) {
					if len(testSuites) != 1 {
						t.Errorf("Unexpected number of test suites, got %d, want %d", len(testSuites), 1)
					}

					files := testSuites[0].TestFiles
					if len(files) != 1 {
						t.Errorf("Unexpected number of test files, got %d, want %d", len(testSuites), 1)
					}

					if !strings.Contains(files[0], "foo.js") {
						t.Errorf("Unexpected test file, got %v, want %v", files[0], "foo.js")
					}
					return "123", nil
				}},
				testFiles: []string{dir.Path()},
				suites: []config.Suite{{
					Name:  "default",
					Match: ".*.js",
				}},
			},
			want:    "123",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Register(tt.args.ctx, tt.args.seq, "", tt.args.testFiles, tt.args.suites)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Register() got = %v, want %v", got, tt.want)
			}
		})
	}
}
