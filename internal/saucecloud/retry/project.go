package retry

import "github.com/saucelabs/saucectl/internal/saucereport"

type Project interface {
	FilterFailedTests(suiteName string, report saucereport.SauceReport) error
}
