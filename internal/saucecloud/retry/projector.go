package retry

import "github.com/saucelabs/saucectl/internal/saucereport"

type Project interface {
	FilterFailedTests(suiteIndex int, report saucereport.SauceReport) error
}
