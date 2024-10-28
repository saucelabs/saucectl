package retry

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"golang.org/x/exp/maps"
)

type JunitRetrier struct {
	JobService job.Service
}

func (b *JunitRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	var tests []string

	if opt.SmartRetry.FailedOnly {
		tests = b.retryFailedTests(&opt, previous)
		if len(tests) == 0 {
			log.Info().Msg(msg.SkippingSmartRetries)
		}
	}

	lg := log.Info().
		Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1))

	if len(tests) > 0 {
		lg.Msgf(msg.RetryWithTests, tests)
	} else {
		lg.Msg("Retrying suite.")
	}

	jobOpts <- opt
}

func (b *JunitRetrier) retryFailedTests(opt *job.StartOptions, previous job.Job) []string {
	if previous.Status == job.StateError {
		log.Warn().Msg(msg.UnreliableReport)
		return nil
	}

	content, err := b.JobService.Artifact(
		context.Background(), previous.ID, junit.FileName, previous.IsRDC,
	)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToFetchFile, junit.FileName)
		return nil
	}

	suites, err := junit.Parse(content)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToUnmarshallFile, junit.FileName)
		return nil
	}

	return setClassesToRetry(opt, suites.TestCases())
}

// setClassesToRetry sets the correct filtering flag when retrying.
// RDC API does not provide different endpoints (or identical values) for Espresso
// and XCUITest. Thus, we need set the classes at the correct position depending
// on the framework that is being executed.
func setClassesToRetry(opt *job.StartOptions, testcases []junit.TestCase) []string {
	if opt.TestOptions == nil {
		opt.TestOptions = map[string]interface{}{}
	}

	if opt.Framework == xcuitest.Kind {
		tests := getFailedXCUITests(testcases, opt.RealDevice)

		// RDC and VDC API filter use different fields for test filtering.
		if opt.RealDevice {
			opt.TestsToRun = tests
		} else {
			opt.TestOptions["class"] = tests
		}

		return tests
	}

	tests := getFailedEspressoTests(testcases)
	opt.TestOptions["class"] = tests

	return tests
}

// getFailedXCUITests returns a list of failed XCUITest tests from the given
// test cases. The format is "<className>/<testMethodName>", with the test
// method name being optional.
func getFailedXCUITests(testCases []junit.TestCase, rdc bool) []string {
	classes := map[string]bool{}
	for _, tc := range testCases {
		if tc.Error != nil || tc.Failure != nil {
			className := conformXCUITestClassName(tc.ClassName, rdc)
			if tc.Name != "" {
				classes[fmt.Sprintf("%s/%s", className, tc.Name)] = true
			} else {
				classes[className] = true
			}
		}
	}
	return maps.Keys(classes)
}

// conformXCUITestClassName conforms the class name of an XCUITest to either
// RDC or VMD. The class name within the platform generated JUnit XML file can
// be dot-separated, but unlike RDC, VMD expects a slash-separated class name.
// The platform is unfortunately not consistent in this regard and is not in
// full control of the generated JUnit XML file.
// If the test is run on RDC, the class name is not modified and returned as is.
// If the test is run on VMD, the class name is converted to a slash-separated.
func conformXCUITestClassName(name string, rdc bool) string {
	if rdc {
		return name
	}

	items := strings.Split(name, ".")
	if len(items) == 1 {
		return name
	}
	return strings.Join(items, "/")
}

// getFailedEspressoTests returns a list of failed Espresso tests from the given
// test cases. The format is "<className>#<testMethodName>", with the test
// method name being optional.
func getFailedEspressoTests(testCases []junit.TestCase) []string {
	classes := map[string]bool{}
	for _, tc := range testCases {
		if tc.Error != nil || tc.Failure != nil {
			if tc.Name != "" {
				classes[fmt.Sprintf("%s#%s", tc.ClassName, tc.Name)] = true
			} else {
				classes[tc.ClassName] = true
			}
		}
	}
	return maps.Keys(classes)
}
