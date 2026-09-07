package authoring

import (
	"strings"

	"github.com/saucelabs/saucectl/internal/region"
)

// JobURL derives a job's dashboard link from its Sauce job identifier.
//
// The service returns a url field on only about half of its jobs (research
// R-006), so the link is always derived instead. This lives here, rather than
// in each caller, because the runner's results table and the `authoring
// testcases run` / `get-run` tables must never disagree about where a job
// lives.
func JobURL(reg region.Region, sauceJobID string) string {
	if sauceJobID == "" {
		return ""
	}
	return reg.AppBaseURL() + "/tests/" + sauceJobID
}

// DescribeCapabilities pulls the browser, platform and device out of a
// target's free-form W3C capabilities.
//
// Capabilities are passed through untouched, so the only way to describe a
// target is to look for the keys that carry meaning, including the appium:
// prefixed forms used for devices. Callers format the three parts however
// their surface needs; keeping the extraction in one place means a new key
// is added once rather than in every table that renders a target.
func DescribeCapabilities(caps map[string]any) (browser, platform, device string) {
	str := func(keys ...string) string {
		for _, k := range keys {
			if s, ok := caps[k].(string); ok && s != "" {
				return s
			}
		}
		return ""
	}
	browser = strings.TrimSpace(str("browserName") + " " + str("browserVersion"))
	platform = strings.TrimSpace(str("platformName") + " " + str("appium:platformVersion", "platformVersion"))
	device = str("appium:deviceName", "deviceName")
	return browser, platform, device
}
