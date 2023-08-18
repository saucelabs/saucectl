package retry

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

type JunitRetrier struct {
	RDCReader job.Reader
	VDCReader job.Reader
}

func (b *JunitRetrier) retryFailedTests(reader job.Reader, jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	content, err := reader.GetJobAssetFileContent(context.Background(), previous.ID, junit.FileName, previous.IsRDC)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToFetchFile, junit.FileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}
	suites, err := junit.Parse(content)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToUnmarshallFile, junit.FileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}

	setClassesToRetry(&opt, junit.CollectTestCases(suites))
	jobOpts <- opt
}

// setClassesToRetry sets the correct filtering flag when retrying.
// RDC API does not provide different endpoints (or identical values) for Espresso
// and XCUITest. Thus, we need set the classes at the correct position depending the
// framework that is being executed.
func setClassesToRetry(opt *job.StartOptions, testcases []junit.TestCase) {
	lg := log.Info().
		Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1))

	if opt.Framework == xcuitest.Kind {
		opt.TestsToRun = getFailedXCUITests(testcases)
		lg.Msgf(msg.RetryWithTests, opt.TestsToRun)
		return
	}

	if opt.TestOptions == nil {
		opt.TestOptions = map[string]interface{}{}
	}
	tests := getFailedEspressoTests(testcases)
	opt.TestOptions["class"] = tests
	lg.Msgf(msg.RetryWithTests, tests)
}

func (b *JunitRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if b.RDCReader != nil && previous.IsRDC && opt.SmartRetry.FailedOnly {
		b.retryFailedTests(b.RDCReader, jobOpts, opt, previous)
		return
	}

	if b.VDCReader != nil && !previous.IsRDC && opt.SmartRetry.FailedOnly {
		b.retryFailedTests(b.VDCReader, jobOpts, opt, previous)
		return
	}

	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}

// getFailedXCUITests get failed XCUITest test list from testcases.
func getFailedXCUITests(testCases []junit.TestCase) []string {
	classes := map[string]bool{}
	for _, tc := range testCases {
		if tc.Error != nil || tc.Failure != nil {
			// The format of the filtered test is "<className>/<testMethodName>".
			// Fallback to <className> if the test method name is unexpectedly empty.
			// tc.Name: <testMethodName>
			// tc.ClassName: <className>
			if tc.Name != "" {
				classes[fmt.Sprintf("%s/%s", tc.ClassName, tc.Name)] = true
			} else {
				classes[tc.ClassName] = true
			}
		}
	}
	return getKeysFromMap(classes)
}

// getFailedEspressoTests get failed espresso test list from testcases.
func getFailedEspressoTests(testCases []junit.TestCase) []string {
	classes := map[string]bool{}
	for _, tc := range testCases {
		if tc.Error != nil || tc.Failure != nil {
			// The format of the filtered test is "<className>#<testMethodName>".
			// Fallback to <className> if the test method name is unexpectedly empty.
			// tc.Name: <testMethodName>
			// tc.ClassName: <className>
			if tc.Name != "" {
				classes[fmt.Sprintf("%s#%s", tc.ClassName, tc.Name)] = true
			} else {
				classes[tc.ClassName] = true
			}
		}
	}
	return getKeysFromMap(classes)
}

func getKeysFromMap(mp map[string]bool) []string {
	var keys = make([]string, len(mp))
	var i int
	for k := range mp {
		keys[i] = k
		i++
	}
	return keys
}
