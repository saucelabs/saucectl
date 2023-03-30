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

type EspressoRetrier struct {
	RDCReader job.Reader
	VDCReader job.Reader
}

func (b *EspressoRetrier) retryOnlyFailedClasses(reader job.Reader, jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	content, err := reader.GetJobAssetFileContent(context.Background(), previous.ID, junit.JunitFileName, previous.IsRDC)
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

func (b *EspressoRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if !opt.SmartRetry.FailedClassesOnly {
		log.Info().Str("suite", opt.DisplayName).
			Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
			Msg("Retrying suite.")
		jobOpts <- opt
		return
	}

	if previous.IsRDC {
		b.retryOnlyFailedClasses(b.RDCReader, jobOpts, opt, previous)
	} else {
		b.retryOnlyFailedClasses(b.VDCReader, jobOpts, opt, previous)
	}
}
