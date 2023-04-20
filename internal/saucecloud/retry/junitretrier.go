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
)

type JunitRetrier struct {
	RDCReader job.Reader
	VDCReader job.Reader
}

func getKeysFromMap(mp map[string]bool) []string {
	keys := make([]string, len(mp))

	i := 0
	for k := range mp {
		keys[i] = k
		i++
	}
	return keys
}

func getFailedClasses(report junit.TestSuites) []string {
	classes := map[string]bool{}

	for _, s := range report.TestSuites {
		for _, tc := range s.TestCases {
			if tc.Error != "" || tc.Failure != "" {
				classes[tc.ClassName] = true
			}
		}
	}
	return getKeysFromMap(classes)
}

func (b *JunitRetrier) retryOnlyFailedClasses(reader job.Reader, jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	content, err := reader.GetJobAssetFileContent(context.Background(), previous.ID, junit.JunitFileName, previous.IsRDC)
	if err != nil {
		log.Debug().Err(err).Msgf(msg.UnableToFetchFile, junit.JunitFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}
	suites, err := junit.Parse(content)
	if err != nil {
		log.Debug().Err(err).Msgf(msg.UnableToUnmarshallFile, junit.JunitFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
		return
	}

	classes := getFailedClasses(suites)
	log.Info().
		Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msgf(msg.RetryWithClasses, strings.Join(classes, ","))

	setClassesToRetry(&opt, classes)

	jobOpts <- opt
}

// RDC API does not provide different endpoints (or identical values) for Espresso
// and XCUITest. Thus, we need set the classes at the correct position depending the
// framework that is being executed.
func setClassesToRetry(opt *job.StartOptions, classes []string) {
	if opt.Framework == xcuitest.Kind {
		opt.TestsToRun = classes
		return
	}
	if opt.TestOptions == nil {
		opt.TestOptions = map[string]interface{}{}
	}
	opt.TestOptions["class"] = classes
}

func (b *JunitRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if b.RDCReader != nil && previous.IsRDC && opt.SmartRetry.FailedClassesOnly {
		b.retryOnlyFailedClasses(b.RDCReader, jobOpts, opt, previous)
		return
	}

	if b.VDCReader != nil && !previous.IsRDC && opt.SmartRetry.FailedClassesOnly {
		b.retryOnlyFailedClasses(b.VDCReader, jobOpts, opt, previous)
		return
	}

	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}
