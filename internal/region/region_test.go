package region

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"

	"github.com/saucelabs/saucectl/internal/iam"
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
			name: "us-east-4",
			args: args{"us-east-4"},
			want: USEast4,
		},
		{
			name: "asia-south-2",
			args: args{"asia-south-2"},
			want: AsiaSouth2,
		},
		{
			name: "staging",
			args: args{"staging"},
			want: Staging,
		},
		{
			name: "wonderland",
			args: args{"wonderland"},
			want: None,
		},
		{
			// asia-south-1 is GCP Mumbai, a plausible typo for the India region.
			// It must not resolve to a region with silently empty URLs.
			name: "asia-south-1",
			args: args{"asia-south-1"},
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

// TestRegionURLs guards the sauceRegionMetas table. Its entries are unkeyed
// struct literals, so a field written out of order still compiles and would
// silently point the CLI at the wrong host.
func TestRegionURLs(t *testing.T) {
	// init() may have loaded ~/.sauce/regions.yml, and user regions override
	// the built-in ones. Isolate so this asserts the built-in table alone.
	saved := userRegionMetas
	userRegionMetas = nil
	t.Cleanup(func() { userRegionMetas = saved })

	tests := []struct {
		region       Region
		apiURL       string
		appURL       string
		webdriverURL string
	}{
		{None, "", "", ""},
		{
			USWest1,
			"https://api.us-west-1.saucelabs.com",
			// us-west-1 predates the per-region app host and keeps the bare one.
			"https://app.saucelabs.com",
			"https://ondemand.us-west-1.saucelabs.com",
		},
		{
			USEast4,
			"https://api.us-east-4.saucelabs.com",
			"https://app.us-east-4.saucelabs.com",
			"https://ondemand.us-east-4.saucelabs.com",
		},
		{
			EUCentral1,
			"https://api.eu-central-1.saucelabs.com",
			"https://app.eu-central-1.saucelabs.com",
			"https://ondemand.eu-central-1.saucelabs.com",
		},
		{
			AsiaSouth2,
			"https://api.asia-south-2.saucelabs.com",
			"https://app.asia-south-2.saucelabs.com",
			"https://ondemand.asia-south-2.saucelabs.com",
		},
		{
			Staging,
			"https://api.staging.saucelabs.net",
			"https://app.staging.saucelabs.net",
			"https://ondemand.staging.saucelabs.net",
		},
	}

	if got := len(allRegionMetas(sauceRegionMetas, nil)); got != len(tests) {
		t.Fatalf("sauceRegionMetas has %d entries but this table covers %d; add the new region's URLs here", got, len(tests))
	}

	for _, tt := range tests {
		t.Run(tt.region.String(), func(t *testing.T) {
			if got := tt.region.APIBaseURL(); got != tt.apiURL {
				t.Errorf("APIBaseURL() = %q, want %q", got, tt.apiURL)
			}
			if got := tt.region.AppBaseURL(); got != tt.appURL {
				t.Errorf("AppBaseURL() = %q, want %q", got, tt.appURL)
			}
			if got := tt.region.WebDriverBaseURL(); got != tt.webdriverURL {
				t.Errorf("WebDriverBaseURL() = %q, want %q", got, tt.webdriverURL)
			}
		})
	}
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
