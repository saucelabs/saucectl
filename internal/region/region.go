package region

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func init() {
	homeDir, _ := os.UserHomeDir()
	path := filepath.Join(homeDir, ".sauce", "regions.yml")
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	if err = yaml.NewDecoder(file).Decode(&userRegionMetas); err != nil {
		return
	}
}

// Region represents the sauce labs region.
type Region string

type regionMeta struct {
	Name             string `yaml:"name"`
	APIBaseURL       string `yaml:"apiBaseURL"`
	AppBaseURL       string `yaml:"appBaseURL"`
	WebdriverBaseURL string `yaml:"webdriverBaseURL"`
}

// None is an undefined sauce labs region.
const None Region = ""

// USWest1 is a sauce labs region in western US, aka us-west-1.
const USWest1 Region = "us-west-1"

// USEast4 is a sauce labs region in eastern US, aka us-east-4.
const USEast4 Region = "us-east-4"

// EUCentral1 is a sauce labs region in the EU, aka eu-central-1.
const EUCentral1 Region = "eu-central-1"

// Staging is a sauce labs internal pre-production environment.
const Staging Region = "staging"

var sauceRegionMetas = []regionMeta{
	{
		None.String(),
		"",
		"",
		"",
	},
	{
		USWest1.String(),
		"https://api.us-west-1.saucelabs.com",
		"https://app.saucelabs.com",
		"https://ondemand.us-west-1.saucelabs.com",
	},
	{
		USEast4.String(),
		"https://api.us-east-4.saucelabs.com",
		"https://app.us-east-4.saucelabs.com",
		"https://ondemand.us-east-4.saucelabs.com",
	},
	{
		EUCentral1.String(),
		"https://api.eu-central-1.saucelabs.com",
		"https://app.eu-central-1.saucelabs.com",
		"https://ondemand.eu-central-1.saucelabs.com",
	},
	{
		Staging.String(),
		"https://api.staging.saucelabs.net",
		"https://app.staging.saucelabs.net",
		"https://ondemand.staging.saucelabs.net",
	},
}

// userRegionMetas is a list of user defined regions that is loaded
// from the user's ~/.sauce directory.
var userRegionMetas []regionMeta

// allRegionMetas concats the list of known Sauce region metadata and the user's
// list of region metadata.
func allRegionMetas() []regionMeta {
	return append(sauceRegionMetas, userRegionMetas...)
}

func (r Region) String() string {
	return string(r)
}

// FromString converts the given string to the corresponding Region.
// Returns None if the string did not match any Region.
func FromString(s string) Region {
	for _, m := range allRegionMetas() {
		if s == m.Name {
			return Region(m.Name)
		}
	}
	return None
}

func lookupMeta(r Region) regionMeta {
	var found regionMeta
	for _, m := range allRegionMetas() {
		if m.Name == string(r) {
			found = m
			break
		}
	}
	return found
}

// APIBaseURL returns the API base URL for the region.
func (r Region) APIBaseURL() string {
	meta := lookupMeta(r)
	return meta.APIBaseURL
}

// AppBaseURL returns the Aapp base URL for the region.
func (r Region) AppBaseURL() string {
	meta := lookupMeta(r)
	return meta.AppBaseURL
}

// WebDriverBaseURL returns the webdriver base URL for the region.
func (r Region) WebDriverBaseURL() string {
	meta := lookupMeta(r)
	return meta.WebdriverBaseURL
}
