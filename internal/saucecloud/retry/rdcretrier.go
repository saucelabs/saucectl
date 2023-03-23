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

var TestOptionsToCopy = map[string][]string{
	"espresso": {"numShards", "shardIndex", "clearPackageData", "useTestOrchestrator"},
}

type RDCRetrier struct {
	Kind      string
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
		if s.Errors == 0 && s.Failures == 0 {
			continue
		}
		for _, tc := range s.TestCases {
			if tc.Error != "" || tc.Failure != "" {
				classes[tc.ClassName] = true
			}
		}
	}
	return getKeysFromMap(classes)
}

func (b *RDCRetrier) keepOptions(testOptions map[string]interface{}) map[string]interface{} {
	val, ok := TestOptionsToCopy[b.Kind]
	if !ok {
		return testOptions
	}

	newTestOpts := map[string]interface{}{}
	for _, k := range val {
		newTestOpts[k] = testOptions[k]
	}
	return newTestOpts
}

func (b *RDCRetrier) smartRDCRetry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
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

	opt.TestOptions = b.keepOptions(opt.TestOptions)

	jobOpts <- opt
}

func (b *RDCRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if previous.IsRDC && opt.SmartRetry {
		b.smartRDCRetry(jobOpts, opt, previous)
		return
	}

	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}
