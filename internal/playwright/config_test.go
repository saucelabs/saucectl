package playwright

import (
	"reflect"
	"testing"

	"github.com/saucelabs/saucectl/internal/config"
)

func Test_shardSuites(t *testing.T) {
	type args struct {
		suites []Suite
	}
	tests := []struct {
		name string
		args args
		want []Suite
	}{
		{
			name: "shard into three",
			args: args{[]Suite{{Name: "Test", NumShards: 3}}},
			want: []Suite{
				{Name: "Test (shard 1/3)", NumShards: 3, Params: SuiteConfig{Shard: "1/3"}},
				{Name: "Test (shard 2/3)", NumShards: 3, Params: SuiteConfig{Shard: "2/3"}},
				{Name: "Test (shard 3/3)", NumShards: 3, Params: SuiteConfig{Shard: "3/3"}},
			},
		},
		{
			name: "shard some",
			args: args{[]Suite{
				{Name: "Test", NumShards: 3},
				{Name: "Unsharded"},
			}},
			want: []Suite{
				{Name: "Test (shard 1/3)", NumShards: 3, Params: SuiteConfig{Shard: "1/3"}},
				{Name: "Test (shard 2/3)", NumShards: 3, Params: SuiteConfig{Shard: "2/3"}},
				{Name: "Test (shard 3/3)", NumShards: 3, Params: SuiteConfig{Shard: "3/3"}},
				{Name: "Unsharded"},
			},
		},
		{
			name: "shard nothing",
			args: args{[]Suite{
				{Name: "Test"},
				{Name: "Test", NumShards: 1},
			}},
			want: []Suite{
				{Name: "Test"},
				{Name: "Test", NumShards: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shardSuitesByNumShards(tt.args.suites); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shardSuites() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name    string
		p       Project
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing version",
			p:       Project{Playwright: Playwright{Version: "v"}},
			wantErr: true,
			errMsg:  "missing framework version. Check available versions here: https://docs.staging.saucelabs.net/testrunner-toolkit#supported-frameworks-and-browsers",
		},
		{
			name: "unable to locate the rootDir folder",
			p: Project{
				Playwright: Playwright{Version: "v1.1.1"}, RootDir: "/test",
			},
			wantErr: true,
			errMsg:  "unable to locate the rootDir folder /test",
		},
		{
			name: "not supported browser",
			p: Project{
				Playwright: Playwright{Version: "v1.1.1"},
				Suites: []Suite{
					{Params: SuiteConfig{BrowserName: "ie"}},
				}},
			wantErr: true,
			errMsg:  "browserName: ie is not supported. List of supported browsers: chromium, firefox, webkit",
		},
		{
			name: "empty region",
			p: Project{
				Sauce:      config.SauceConfig{Region: ""},
				Playwright: Playwright{Version: "v1.1.1"},
				Suites: []Suite{
					{Name: "suite #1", NumShards: 2, Params: SuiteConfig{BrowserName: "chromium"}},
				}},
			wantErr: true,
			errMsg:  "no sauce region set",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				err := Validate(&tt.p)
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() = %v, want %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
