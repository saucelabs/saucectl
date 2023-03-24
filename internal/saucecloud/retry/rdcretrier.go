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

type RDCRetrier struct {
	RDCReader job.Reader
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

func (b *RDCRetrier) retryOnlyFailedClasses(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	content, err := b.RDCReader.GetJobAssetFileContent(context.Background(), previous.ID, junit.JunitFileName, previous.IsRDC)
	if err != nil {
		log.Debug().Err(err).Msgf(msg.UnableToFetchFile, junit.JunitFileName)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
	}
	suites, err := junit.Parse(content)
	if err != nil {
		log.Debug().Err(err).Msg(msg.UnableToUnmarshallFile)
		log.Info().Msg(msg.SkippingSmartRetries)
		jobOpts <- opt
	}

	classes := getFailedClasses(suites)
	log.Info().Msgf(msg.RetryWithClasses, strings.Join(classes, ","))

	opt.TestOptions["class"] = classes
	jobOpts <- opt
}

func (b *RDCRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if previous.IsRDC && opt.SmartRetry.FailedClassesOnly {
		b.retryOnlyFailedClasses(jobOpts, opt, previous)
		return
	}

	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}
