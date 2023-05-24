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
	content, err := reader.GetJobAssetFileContent(context.Background(), previous.ID, junit.JunitFileName, previous.IsRDC)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToFetchFile, junit.JunitFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}
	suites, err := junit.Parse(content)
	if err != nil {
		log.Err(err).Msgf(msg.UnableToUnmarshallFile, junit.JunitFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}

	setClassesToRetry(&opt, suites)
	jobOpts <- opt
}

// setClassesToRetry sets the correct filtering flag when retrying.
// RDC API does not provide different endpoints (or identical values) for Espresso
// and XCUITest. Thus, we need set the classes at the correct position depending the
// framework that is being executed.
func setClassesToRetry(opt *job.StartOptions, suites junit.TestSuites) {
	lg := log.Info().
		Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1))

	if opt.Framework == xcuitest.Kind {
		opt.TestsToRun = junit.GetFailedTests(suites)
		lg.Msgf(msg.RetryWithClasses, opt.TestsToRun)
		return
	}
	if opt.TestOptions == nil {
		opt.TestOptions = map[string]interface{}{}
	}
        classes := junit.GetFailedClasses(suites)
	opt.TestOptions["class"] = classes
	lg.Msgf(msg.RetryWithClasses, classes)
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
