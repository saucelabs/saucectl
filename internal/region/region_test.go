package region

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestFromString(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want Region
	}{
		{
			name: "us-west-1",
			args: args{"us-west-1"},
			want: USWest1,
		},
		{
			name: "eu-central-1",
			args: args{"eu-central-1"},
			want: EUCentral1,
		},
		{
			name: "apac-southeast-1",
			args: args{"apac-southeast-1"},
			want: APACSoutheast1,
		},
		{
			name: "wonderland",
			args: args{"wonderland"},
			want: None,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromString(tt.args.s); got != tt.want {
				t.Errorf("FromString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	name := "staging"
	r := FromString(name)
	assert.Equal(t, name, r.String())
}
