package region

// Region represents the sauce labs region.
type Region uint

const (
	// None is an undefined sauce labs region.
	None Region = iota
	// USWest1 is a sauce labs region in the US, aka us-west-1.
	USWest1
	// EUCentral1 is a sauce labs region in the EU, aka eu-central-1.
	EUCentral1
	// APACSoutheast is a sauce labs region in the APAC, aka apac-southeast-1
	APACSoutheast
	// Staging is a sauce labs internal pre-production environment.
	Staging
)

var meta = []struct {
	Name       string
	APIBaseURL string
	AppBaseURL string
}{
	// None
	{
		"",
		"",
		"",
	},
	// USWest1
	{
		"us-west-1",
		"https://api.us-west-1.saucelabs.com",
		"https://app.saucelabs.com",
	},
	// EUCentral1
	{
		"eu-central-1",
		"https://api.eu-central-1.saucelabs.com",
		"https://app.eu-central-1.saucelabs.com",
	},
	// APAC
	{
		"apac-southeast-1",
		"https://api.apac-southeast-1.saucelabs.com",
		"https://api.apac-southeast-1.saucelabs.com",
	},
	// Staging
	{
		"staging",
		"https://api.staging.saucelabs.net",
		"https://app.staging.saucelabs.net",
	},
}

func (r Region) String() string {
	return meta[r].Name
}

// FromString converts the given string to the corresponding Region.
// Returns None if the string did not match any Region.
func FromString(s string) Region {
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
