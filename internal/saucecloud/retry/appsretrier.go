package retry

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/msg"
	"strings"
)

type AppsRetrier struct {
	RDCReader job.Reader
	VDCReader job.Reader

	RetryRDC bool
	RetryVDC bool
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

func (b *AppsRetrier) retryOnlyFailedClasses(reader job.Reader, jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
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

	opt.TestOptions["class"] = classes
	jobOpts <- opt
}

func (b *AppsRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if b.RetryRDC && previous.IsRDC && opt.SmartRetry.FailedClassesOnly {
		b.retryOnlyFailedClasses(b.RDCReader, jobOpts, opt, previous)
		return
	}

	if b.RetryVDC && !previous.IsRDC && opt.SmartRetry.FailedClassesOnly {
		b.retryOnlyFailedClasses(b.VDCReader, jobOpts, opt, previous)
		return
	}

	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}
