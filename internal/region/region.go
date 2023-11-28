package region

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/iam"
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
		log.Error().Msgf("failed to parse regions file (%s): %v", path, err)
		return
	}
}

// Region represents the sauce labs region.
type Region string

type regionMeta struct {
	Name             string          `yaml:"name"`
	APIBaseURL       string          `yaml:"apiBaseURL"`
	AppBaseURL       string          `yaml:"appBaseURL"`
	WebdriverBaseURL string          `yaml:"webdriverBaseURL"`
	Credentials      iam.Credentials `yaml:"credentials"`
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

var defaultCreds = credentials.Get()

var sauceRegionMetas = []regionMeta{
	{
		None.String(),
		"",
		"",
		"",
		iam.Credentials{},
	},
	{
		USWest1.String(),
		"https://api.us-west-1.saucelabs.com",
		"https://app.saucelabs.com",
		"https://ondemand.us-west-1.saucelabs.com",
		defaultCreds,
	},
	{
		USEast4.String(),
		"https://api.us-east-4.saucelabs.com",
		"https://app.us-east-4.saucelabs.com",
		"https://ondemand.us-east-4.saucelabs.com",
		defaultCreds,
	},
	{
		EUCentral1.String(),
		"https://api.eu-central-1.saucelabs.com",
		"https://app.eu-central-1.saucelabs.com",
		"https://ondemand.eu-central-1.saucelabs.com",
		defaultCreds,
	},
	{
		Staging.String(),
		"https://api.staging.saucelabs.net",
		"https://app.staging.saucelabs.net",
		"https://ondemand.staging.saucelabs.net",
		defaultCreds,
	},
}

// userRegionMetas is a list of user defined regions that is loaded
// from the user's ~/.sauce directory.
var userRegionMetas []regionMeta

func mergeRegionMetas(base regionMeta, overlay regionMeta) regionMeta {
	merged := base
	if overlay.Name != "" {
		merged.Name = overlay.Name
	}
	if overlay.APIBaseURL != "" {
		merged.APIBaseURL = overlay.APIBaseURL
	}
	if overlay.AppBaseURL != "" {
		merged.AppBaseURL = overlay.AppBaseURL
	}
	if overlay.WebdriverBaseURL != "" {
		merged.WebdriverBaseURL = overlay.WebdriverBaseURL
	}
	if overlay.Credentials.IsSet() {
		merged.Credentials = overlay.Credentials
	}

	return merged
}

// allRegionMetas concats the list of known Sauce region metadata and the user's
// list of region metadata.
func allRegionMetas(sauce []regionMeta, user []regionMeta) map[Region]regionMeta {
	mappedRegions := make(map[Region]regionMeta)
	for _, m := range sauce {
		mappedRegions[Region(m.Name)] = m
	}

	for _, userMeta := range user {
		userRegion := Region(userMeta.Name)

		curr, ok := mappedRegions[userRegion]
		if !ok {
			mappedRegions[userRegion] = userMeta
			continue
		}
		mappedRegions[userRegion] = mergeRegionMetas(curr, userMeta)
	}

	return mappedRegions
}

func (r Region) String() string {
	return string(r)
}

// FromString converts the given string to the corresponding Region.
// Returns None if the string did not match any Region.
func FromString(s string) Region {
	_, ok := allRegionMetas(sauceRegionMetas, userRegionMetas)[Region(s)]
	if ok {
		return Region(s)
	}
	return None
}

func lookupMeta(r Region) regionMeta {
	m, ok := allRegionMetas(sauceRegionMetas, userRegionMetas)[r]
	if ok {
		return m
	}
	return regionMeta{}
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

func (r Region) Credentials() iam.Credentials {
	meta := lookupMeta(r)
        // check if there are any region specific credentials first
	if meta.Credentials.IsSet() {
		return meta.Credentials
	}

	return defaultCreds
}
