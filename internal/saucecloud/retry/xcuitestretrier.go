package retry

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"strings"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
)

type XCUITestRetrier struct {
	RDCReader job.Reader
}

//
//func getFailedClasses(report junit.TestSuites) []string {
//	classes := map[string]bool{}
//
//	for _, s := range report.TestSuites {
//		if s.Errors == 0 && s.Failures == 0 {
//			continue
//		}
//		for _, tc := range s.TestCases {
//			if tc.Error != "" || tc.Failure != "" {
//				classes[tc.ClassName] = true
//			}
//		}
//	}
//	return maps.Keys(classes)
//}

// FIXME: Correct error messages
func (b *XCUITestRetrier) smartRDCRetry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	content, err := b.RDCReader.GetJobAssetFileContent(context.Background(), previous.ID, junit.JunitFileName, previous.IsRDC)
	if err != nil {
		log.Err(err).Msg("1- unable to determine which suites to restart. Retrying complete suite")
		jobOpts <- opt
	}
	suites, err := junit.Parse(content)
	if err != nil {
		log.Err(err).Msg("2- unable to determine which suites to restart. Retrying complete suite")
		jobOpts <- opt
	}

	classes := getFailedClasses(suites)
	log.Info().Str("classes", strings.Join(classes, ",")).Msg("Restarting only failing classes")

	opt.TestOptions = map[string]interface{}{}
	opt.TestOptions["class"] = classes

	jobOpts <- opt
	return
}

func (b *XCUITestRetrier) Retry(jobOpts chan<- job.StartOptions, opt job.StartOptions, previous job.Job) {
	if previous.IsRDC && opt.SmartRetry {
		b.smartRDCRetry(jobOpts, opt, previous)
		return
	}

	log.Info().Str("suite", opt.DisplayName).
		Str("attempt", fmt.Sprintf("%d of %d", opt.Attempt+1, opt.Retries+1)).
		Msg("Retrying suite.")
	jobOpts <- opt
}
