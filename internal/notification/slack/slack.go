package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/region"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/saucelabs/saucectl/internal/report"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
)

// Reporter represents reporter for slack
type Reporter struct {
	Channels       []string
	TestResults    []report.TestResult
	Framework      string
	Metadata       config.Metadata
	TestEnv        string
	RenderedResult string
	Config         config.Notifications
	Service        Service
	lock           sync.Mutex
}

// Add adds the TestResult to the reporter. TestResults added this way can then be rendered out by calling Render().
func (r *Reporter) Add(t report.TestResult) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.TestResults = append(r.TestResults, t)
}

// Render renders the test results and sends message to the Slack.
func (r *Reporter) Render() {
	r.lock.Lock()
	defer r.lock.Unlock()

	tables := [][]string{}
	longestName := 0
	for _, ts := range r.TestResults {
		if longestName < len(ts.Name) {
			longestName = len(ts.Name)
		}
	}

	passed := true
	for _, ts := range r.TestResults {
		url := ts.URL + "?utm_source=slack&utm_medium=chat&utm_campaign=testresults"
		tables = append(tables, []string{ts.Status, addRightSpaces(ts.Name, r.getJobURL(ts.Name, url), longestName),
			ts.Platform, ts.DeviceName, ts.Browser, ts.Duration.Truncate(1 * time.Second).String()})
		if ts.Status != job.StatePassed {
			passed = false
		}
	}
	var res string
	for _, t := range tables {
		res = fmt.Sprintf("%s\n%s", res, strings.Join(t, "\t"))
	}

	r.RenderedResult = res

	r.sendMessage(passed)
}

// GetRenderedResult returns rendered result.
func (r *Reporter) GetRenderedResult() string {
	return r.RenderedResult
}

// Reset no need to implement
func (r *Reporter) Reset() {}

// ArtifactRequirements no need to implement
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return nil
}

// sendMessage sends notification message.
func (r *Reporter) sendMessage(passed bool) {
	if !r.shouldSendNotification(passed) {
		return
	}

	token, err := r.Service.GetSlackToken(context.Background())
	if err != nil {
		log.Err(err).Msg("Failed to get slack token")
		return
	}

	var sentNotifications int
	var failedNotifications int

	api := slack.New(token)

	for _, c := range r.Channels {
		_, _, err := api.PostMessage(
			c,
			slack.MsgOptionText("saucectl test result", false),
			slack.MsgOptionAttachments(r.creatAttachment(passed)),
			slack.MsgOptionAsUser(true),
		)

		if err != nil {
			log.Error().Msgf("Failed to send message to slack, err: %s", err.Error())
			failedNotifications++
			continue
		}
		sentNotifications++

		log.Info().Msgf("Message successfully sent to slack channel %s", c)
	}
}

// shouldSendNotification returns true if it should send notification, otherwise false
func (r *Reporter) shouldSendNotification(passed bool) bool {
	if len(r.Config.Slack.Channels) == 0 ||
		r.Config.Slack.Send == config.WhenNever {
		return false
	}

	if r.Config.Slack.Send == config.WhenAlways ||
		(r.Config.Slack.Send == config.WhenFail && !passed) ||
		(passed && r.Config.Slack.Send == config.WhenPass) {
		return true
	}

	return false
}

func (r *Reporter) createBlocks() []slack.Block {
	contextText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s | *Build ID*: %s | %s | %s", r.getFrameworkName(), r.Metadata.Build, credentials.Get(region.None).Username, time.Now().Format("2006-01-02 15:04:05")), false, false)
	contextSection := slack.NewSectionBlock(contextText, nil, nil)

	resultText := slack.NewTextBlockObject("mrkdwn", r.GetRenderedResult(), false, false)
	resultSection := slack.NewSectionBlock(resultText, nil, nil)

	return []slack.Block{contextSection, resultSection}
}

func (r *Reporter) getFrameworkName() string {
	if r.Framework == "xcuitest" {
		return "XCUITest"
	}
	if r.Framework == "testcafe" {
		return "TestCafe"
	}

	return cases.Title(language.English).String(r.Framework)
}

func addRightSpaces(name, wholeName string, length int) string {
	minus := length - len(name)
	for minus > 0 {
		wholeName = fmt.Sprintf("%s%s", wholeName, " ")
		minus--
	}
	return wholeName
}

func (r *Reporter) getJobURL(name, jobURL string) string {
	return fmt.Sprintf("<%s|%s>", jobURL, name)
}

func (r *Reporter) creatAttachment(passed bool) slack.Attachment {
	color := "#F00000"
	if passed {
		color = "#008000"
	}
	return slack.Attachment{
		Color:  color,
		Blocks: slack.Blocks{BlockSet: r.createBlocks()},
	}
}
