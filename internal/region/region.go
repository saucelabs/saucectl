package region

import (
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/clientconfig"
)

// Region represents the sauce labs region.
type Region uint

const (
	// None is an undefined sauce labs region.
	None Region = iota
	// USWest1 is a sauce labs region in western US, aka us-west-1.
	USWest1
	// USEast4 is a sauce labs region in eastern US, aka us-east-4.
	USEast4
	// EUCentral1 is a sauce labs region in the EU, aka eu-central-1.
	EUCentral1
	// Staging is a sauce labs internal pre-production environment.
	Staging
)

type Data struct {
	Name             string `json:"name"`
	APIBaseURL       string `json:"apiBaseURL,omitempty"`
	AppBaseURL       string `json:"appBaseURL,omitempty"`
	WebDriverBaseURL string `json:"webdriverBaseURL,omitempty"`
}

var meta = []Data{
	// None
	{
		"",
		"",
		"",
		"",
	},
	// USWest1
	{
		"us-west-1",
		"https://api.us-west-1.saucelabs.com",
		"https://app.saucelabs.com",
		"https://ondemand.us-west-1.saucelabs.com",
	},
	// USEast4
	{
		"us-east-4",
		"https://api.us-east-4.saucelabs.com",
		"https://app.us-east-4.saucelabs.com",
		"https://ondemand.us-east-4.saucelabs.com",
	},
	// EUCentral1
	{
		"eu-central-1",
		"https://api.eu-central-1.saucelabs.com",
		"https://app.eu-central-1.saucelabs.com",
		"https://ondemand.eu-central-1.saucelabs.com",
	},
	// Staging
	{
		"staging",
		"https://api.staging.saucelabs.net",
		"https://app.staging.saucelabs.net",
		"https://ondemand.staging.saucelabs.net",
	},
}

var initialized = false

func Init() {
	if initialized {
		return
	}
	clientConf, err := clientconfig.Get()
	if err != nil {
		log.Warn().Err(err).Msg("failed to read client config")
		return
	}
	if clientConf != nil {
		for _, r := range clientConf.Regions {
			found := false
			for i, m := range meta {
				if m.Name == r.Name {
					found = true
					if r.APIBaseURL != "" {
						meta[i].APIBaseURL = r.APIBaseURL
					}
					if r.AppBaseURL != "" {
						meta[i].AppBaseURL = r.AppBaseURL
					}
					if r.WebDriverBaseURL != "" {
						meta[i].WebDriverBaseURL = r.WebDriverBaseURL
					}
					break
				}
			}
			if !found {
				meta = append(meta, Data{
					Name:             r.Name,
					APIBaseURL:       r.APIBaseURL,
					AppBaseURL:       r.AppBaseURL,
					WebDriverBaseURL: r.WebDriverBaseURL,
				})
			}
		}
	}
	initialized = true
}

func (r Region) String() string {
	Init()
	return meta[r].Name
}

// FromString converts the given string to the corresponding Region.
// Returns None if the string did not match any Region.
func FromString(s string) Region {
	Init()
	for i, m := range meta {
		if m.Name == s {
			return Region(i)
		}
	}
	return None
}

// APIBaseURL returns the API base URL for the region.
func (r Region) APIBaseURL() string {
	return meta[r].APIBaseURL
}

// AppBaseURL returns the Aapp base URL for the region.
func (r Region) AppBaseURL() string {
	return meta[r].AppBaseURL
}

// WebDriverBaseURL returns the webdriver base URL for the region.
func (r Region) WebDriverBaseURL() string {
	return meta[r].WebDriverBaseURL
}
