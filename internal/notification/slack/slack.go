package slack

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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
		tables = append(tables, []string{statusText(ts.Passed), regenerateName(ts.Name, r.getJobURL(ts.Name, ts.URL), longestName),
			ts.Platform, ts.DeviceName, ts.Browser, ts.Duration.Truncate(1 * time.Second).String()})
		if !ts.Passed {
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

// Reset resets the state of the reporter (e.g. remove any previously reported TestResults).
func (r *Reporter) Reset() {}

// ArtifactRequirements returns a list of artifact types that this reporter requires to create a proper report.
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

	api := slack.New(token)

	for _, c := range r.Channels {
		_, timestamp, err := api.PostMessage(
			c,
			slack.MsgOptionText("saucectl test result", false),
			slack.MsgOptionAttachments(r.creatAttachment(passed)),
			slack.MsgOptionAsUser(true),
		)

		if err != nil {
			log.Error().Msgf("Failed to send message to slack, err: %s", err.Error())
			continue
		}

		timestampArr := strings.Split(timestamp, ".")
		i, err := strconv.ParseInt(timestampArr[0], 10, 64)
		if err != nil {
			log.Info().Msgf("Couldn't parse slack timestamp, err: %s", err)
		} else {
			i = time.Now().Unix()
		}

		log.Info().Msgf("Message successfully sent to slack channel %s at %s", c, time.Unix(i, 0))
	}
}

// shouldSendNotification returns true if it should send notification, otherwise false
func (r *Reporter) shouldSendNotification(passed bool) bool {
	for _, ts := range r.TestResults {
		if ts.URL == "" {
			return false
		}
	}

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
	headerText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s*", r.Metadata.Build), false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	contextText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s | *Build ID*: %s | %s | %s", r.getFrameworkName(), r.Metadata.Build, credentials.Get().Username, time.Now().Format("2006-01-02 15:04:05")), false, false)
	contextSection := slack.NewSectionBlock(contextText, nil, nil)

	resultText := slack.NewTextBlockObject("mrkdwn", r.GetRenderedResult(), false, false)
	resultSection := slack.NewSectionBlock(resultText, nil, nil)

	blocks := make([]slack.Block, 0)
	blocks = append(blocks, headerSection)
	blocks = append(blocks, contextSection)
	blocks = append(blocks, resultSection)

	return blocks
}

func (r *Reporter) getFrameworkName() string {
	if r.Framework == "xcuitest" {
		return "XCUITest"
	}

	return strings.Title(r.Framework)
}

func regenerateName(name, wholeName string, length int) string {
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

func statusText(passed bool) string {
	if !passed {
		return "failed"
	}
	return "passed"
}
