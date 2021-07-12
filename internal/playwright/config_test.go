package playwright

import (
	"reflect"
	"testing"
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
			args: args{[]Suite{{Name: "Test"}}},
			want: []Suite{
				{Name: "Test"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shardSuites(tt.args.suites); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shardSuites() = %v, want %v", got, tt.want)
			}
		})
	}
}
