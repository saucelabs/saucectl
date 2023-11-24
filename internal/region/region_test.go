package region

import (
	"testing"

	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/google/go-cmp/cmp"
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

func Test_mergeRegionMetas(t *testing.T) {
	type args struct {
		base    regionMeta
		overlay regionMeta
	}
	tests := []struct {
		name string
		args args
		want regionMeta
	}{
		{
			name: "base props get overwritten by overlay",
			args: args{
				base: regionMeta{
					Name:             "Region",
					APIBaseURL:       "api1",
					AppBaseURL:       "app1",
					WebdriverBaseURL: "wd1",
					Credentials: iam.Credentials{
						Username:  "user1",
						AccessKey: "ac1",
					},
				},
				overlay: regionMeta{
					Name:             "Overlay",
					APIBaseURL:       "api2",
					AppBaseURL:       "app2",
					WebdriverBaseURL: "wd2",
					Credentials: iam.Credentials{
						Username:  "user2",
						AccessKey: "ac2",
					},
				},
			},
			want: regionMeta{
				Name:             "Overlay",
				APIBaseURL:       "api2",
				AppBaseURL:       "app2",
				WebdriverBaseURL: "wd2",
				Credentials: iam.Credentials{
					Username:  "user2",
					AccessKey: "ac2",
				},
			},
		},
		{
			name: "empty base creds get set by overlay",
			args: args{
				base: regionMeta{
					Name:             "Region",
					APIBaseURL:       "api1",
					AppBaseURL:       "app1",
					WebdriverBaseURL: "wd1",
				},
				overlay: regionMeta{
					Credentials: iam.Credentials{
						Username:  "user",
						AccessKey: "ac",
					},
				},
			},
			want: regionMeta{
				Name:             "Region",
				APIBaseURL:       "api1",
				AppBaseURL:       "app1",
				WebdriverBaseURL: "wd1",
				Credentials: iam.Credentials{
					Username:  "user",
					AccessKey: "ac",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeRegionMetas(tt.args.base, tt.args.overlay)
			if tt.want != got {
				t.Errorf("mergeRegionMetas() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_allRegionMetas(t *testing.T) {
	type args struct {
		sauce []regionMeta
		user  []regionMeta
	}
	tests := []struct {
		name string
		args args
		want map[Region]regionMeta
	}{
		{
			name: "user regions appended to sauce regions",
			args: args{
				sauce: []regionMeta{
					{
						Name: "Staging",
					},
				},
				user: []regionMeta{
					{
						Name: "Local",
					},
				},
			},
			want: map[Region]regionMeta{
				Region("Staging"): {
					Name: "Staging",
				},
				Region("Local"): {
					Name: "Local",
				},
			},
		},
		{
			name: "user region can overwrite sauce regions",
			args: args{
				sauce: []regionMeta{
					{
						Name: "staging",
						Credentials: iam.Credentials{
							Username:  "default",
							AccessKey: "default",
						},
					},
				},
				user: []regionMeta{
					{
						Name: "staging",
						Credentials: iam.Credentials{
							Username:  "custom",
							AccessKey: "custom",
						},
					},
				},
			},
			want: map[Region]regionMeta{
				Staging: {
					Name: Staging.String(),
					Credentials: iam.Credentials{
						Username:  "custom",
						AccessKey: "custom",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allRegionMetas(tt.args.sauce, tt.args.user)
			if !cmp.Equal(got, tt.want) {
				t.Errorf("allRegionalMetas() = %v, want %v", got, tt.want)

			}
		})
	}
}
