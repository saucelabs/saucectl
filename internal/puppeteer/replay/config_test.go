package replay

import (
	"os"
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/insights"
	"gotest.tools/assert"
	"gotest.tools/v3/fs"
)

func TestShardSuites(t *testing.T) {
	dir := fs.NewDir(t, "testcafe",
		fs.WithDir("tests",
			fs.WithMode(0755),
			fs.WithDir("dir1",
				fs.WithMode(0755),
				fs.WithFile("recording1.json", "", fs.WithMode(0644)),
			),
			fs.WithDir("dir2",
				fs.WithMode(0755),
				fs.WithFile("recording2.json", "", fs.WithMode(0644)),
			),
			fs.WithDir("dir3",
				fs.WithMode(0755),
				fs.WithFile("recording3.json", "", fs.WithMode(0644)),
			),
			fs.WithDir("dir4",
				fs.WithMode(0755),
				fs.WithFile("trap.xml", "", fs.WithMode(0644)),
			),
		),
	)
	defer dir.Remove()

	type args struct {
		suites []Suite
	}
	tests := []struct {
		name    string
		args    args
		want    []Suite
		wantErr bool
	}{
		{
			name: "single recording",
			args: args{
				suites: []Suite{
					{
						Name:       "suite #1",
						Recordings: []string{"tests/dir1/recording1.json"},
					},
				}},
			wantErr: false,
			want: []Suite{
				{Name: "suite #1 - tests/dir1/recording1.json", Recording: "tests/dir1/recording1.json", Recordings: []string{"tests/dir1/recording1.json"}},
			},
		},
		{
			name: "split by recording",
			args: args{
				suites: []Suite{
					{
						Name:       "suite #1",
						Recordings: []string{"**/recording*.json"},
					},
				}},
			wantErr: false,
			want: []Suite{
				{Name: "suite #1 - tests/dir1/recording1.json", Recording: "tests/dir1/recording1.json", Recordings: []string{"**/recording*.json"}},
				{Name: "suite #1 - tests/dir2/recording2.json", Recording: "tests/dir2/recording2.json", Recordings: []string{"**/recording*.json"}},
				{Name: "suite #1 - tests/dir3/recording3.json", Recording: "tests/dir3/recording3.json", Recordings: []string{"**/recording*.json"}},
			},
		},
		{
			name: "no matching recordings",
			args: args{
				suites: []Suite{
					{
						Name:       "suite #1",
						Recordings: []string{"troll.json"},
					},
				}},
			wantErr: true,
		},
		{
			name: "non-json recordings",
			args: args{
				suites: []Suite{
					{
						Name:       "suite #1",
						Recordings: []string{"tests/dir4/trap.xml"},
					},
				}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.Chdir(dir.Path())
			if err != nil {
				t.Errorf("Chdir() error = %v", err)
			}

			got, err := ShardSuites(tt.args.suites)
			if (err != nil) != tt.wantErr {
				t.Errorf("ShardSuites() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ShardSuites() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterSuites(t *testing.T) {
	type args struct {
		p         *Project
		suiteName string
	}
	tests := []struct {
		name    string
		args    args
		want    []Suite
		wantErr bool
	}{
		{
			name: "filter suite",
			args: args{p: &Project{
				Suites: []Suite{{Name: "suite #1"}, {Name: "suite #2"}},
			},
				suiteName: "suite #1",
			},
			want:    []Suite{{Name: "suite #1"}},
			wantErr: false,
		},
		{
			name: "suite not found",
			args: args{p: &Project{
				Suites: []Suite{{Name: "suite #1"}, {Name: "suite #2"}},
			},
				suiteName: "suite #3",
			},
			want:    []Suite{{Name: "suite #1"}, {Name: "suite #2"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := FilterSuites(tt.args.p, tt.args.suiteName); (err != nil) != tt.wantErr {
				t.Errorf("FilterSuites() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(tt.args.p.Suites, tt.want) {
				t.Errorf("FilterSuites() got = %v, want %v", tt.args.p.Suites, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	type args struct {
		p *Project
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid browsers",
			args: args{p: &Project{
				Sauce: config.SauceConfig{Region: "us-west-1"},
				Suites: []Suite{
					{Name: "suite #1", BrowserName: "chrome"},
					{Name: "suite #2", BrowserName: "googlechrome"},
				}}},
			wantErr: false,
		},
		{
			name:    "invalid browser",
			args:    args{p: &Project{Suites: []Suite{{Name: "suite #1", BrowserName: "firefox"}}}},
			wantErr: true,
		},
		{
			name:    "invalid region",
			args:    args{p: &Project{Sauce: config.SauceConfig{Region: "heaven"}}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.args.p); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReplay_SortByHistory(t *testing.T) {
	testCases := []struct {
		name    string
		suites  []Suite
		history insights.JobHistory
		expRes  []Suite
	}{
		{
			name: "sort suites by job history",
			suites: []Suite{
				Suite{Name: "suite 1"},
				Suite{Name: "suite 2"},
				Suite{Name: "suite 3"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					insights.TestCase{Name: "suite 2"},
					insights.TestCase{Name: "suite 1"},
					insights.TestCase{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				Suite{Name: "suite 2"},
				Suite{Name: "suite 1"},
				Suite{Name: "suite 3"},
			},
		},
		{
			name: "suites is the subset of job history",
			suites: []Suite{
				Suite{Name: "suite 1"},
				Suite{Name: "suite 2"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					insights.TestCase{Name: "suite 2"},
					insights.TestCase{Name: "suite 1"},
					insights.TestCase{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				Suite{Name: "suite 2"},
				Suite{Name: "suite 1"},
			},
		},
		{
			name: "job history is the subset of suites",
			suites: []Suite{
				Suite{Name: "suite 1"},
				Suite{Name: "suite 2"},
				Suite{Name: "suite 3"},
				Suite{Name: "suite 4"},
				Suite{Name: "suite 5"},
			},
			history: insights.JobHistory{
				TestCases: []insights.TestCase{
					insights.TestCase{Name: "suite 2"},
					insights.TestCase{Name: "suite 1"},
					insights.TestCase{Name: "suite 3"},
				},
			},
			expRes: []Suite{
				Suite{Name: "suite 2"},
				Suite{Name: "suite 1"},
				Suite{Name: "suite 3"},
				Suite{Name: "suite 4"},
				Suite{Name: "suite 5"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SortByHistory(tc.suites, tc.history)
			for i := 0; i < len(result); i++ {
				assert.Equal(t, tc.expRes[i].Name, result[i].Name)
			}
		})
	}
}
