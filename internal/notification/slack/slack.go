package slack

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/saucelabs/saucectl/internal/report"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
)

// Notifier represents notifier for slack
type Notifier struct {
	Token          string
	Channels       []string
	TestResults    []report.TestResult
	Framework      string
	Passed         bool
	Metadata       config.Metadata
	TestEnv        string
	RenderedResult string
}

// Add adds the TestResult to the reporter. TestResults added this way can then be rendered out by calling Render().
func (s *Notifier) Add(t report.TestResult) {
	s.TestResults = append(s.TestResults, t)
}

// Render renders the test results. The destination is RenderedResult.
func (s *Notifier) Render() {
	tables := [][]string{}
	longestName := 0
	for _, ts := range s.TestResults {
		if longestName < len(ts.Name) {
			longestName = len(ts.Name)
		}
	}
	header := []string{"Passed", "Name                     ", "Duration", "Status", "Browser", "Platform", "Device"}
	tables = append(tables, header)

	for _, ts := range s.TestResults {
		tables = append(tables, []string{statusSymbol(ts.Passed), regenerateName(ts.Name, s.getJobURL(ts.Name, ts.URL), longestName), ts.Duration.Truncate(1 * time.Second).String(),
			statusText(ts.Passed), ts.Browser, ts.Platform, ts.DeviceName})
	}
	var res string
	for _, t := range tables {
		res = fmt.Sprintf("%s\n%s", res, strings.Join(t, "\t"))
	}

	s.RenderedResult = res
}

// GetRenderedResult returns rendered result.
func (s *Notifier) GetRenderedResult() string {
	s.Render()
	return s.RenderedResult
}

// Reset resets the state of the reporter (e.g. remove any previously reported TestResults).
func (s *Notifier) Reset() {}

// ArtifactRequirements returns a list of artifact types that this reporter requires to create a proper report.
func (s *Notifier) ArtifactRequirements() []report.ArtifactType {
	return nil
}

// SendMessage send notification message.
func (s *Notifier) SendMessage() {
	api := slack.New(s.Token)

	for _, c := range s.Channels {
		_, timestamp, err := api.PostMessage(
			c,
			slack.MsgOptionText("saucectl test result", false),
			slack.MsgOptionAttachments(s.creatAttachment()),
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

// ShouldSendNotification returns true if it should send notification, otherwise false
func (s *Notifier) ShouldSendNotification(cfg config.Notifications) bool {
	for _, ts := range s.TestResults {
		if ts.URL == "" {
			return false
		}
	}

	if len(cfg.Slack.Channels) == 0 || cfg.Slack.Send == config.SendNever {
		return false
	}

	if cfg.Slack.Send == config.SendAlways ||
		(cfg.Slack.Send == config.SendOnFailure && !s.Passed) {
		return true
	}

	return false
}

func (s *Notifier) createBlocks() []slack.Block {
	headerText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s*", s.Metadata.Build), false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	contextElementText := slack.NewImageBlockElement(s.getFrameworkIcon(), "Framework icon")
	contextText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s | *Build ID*: %s | %s | %s", s.getTestEnvEmoji(), s.Metadata.Build, credentials.Get().Username, time.Now().Format("2006-01-02 15:04:05")), false, false)
	frameworkIconSection := slack.NewContextBlock("", []slack.MixedElement{contextElementText, contextText}...)

	resultText := slack.NewTextBlockObject("mrkdwn", s.GetRenderedResult(), false, false)
	resultSection := slack.NewSectionBlock(resultText, nil, nil)

	blocks := make([]slack.Block, 0)
	blocks = append(blocks, headerSection)
	blocks = append(blocks, frameworkIconSection)
	blocks = append(blocks, resultSection)

	return blocks
}

func (s *Notifier) getFrameworkIcon() string {
	switch s.Framework {
	case "cypress":
		return "https://miro.medium.com/max/1200/1*cenjHE5G6nX-8ftK4MuT-A.png"
	case "playwright":
		return "https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcS_BzH2Y-hRybEebvPMQRSRYjtw6vgdiPandQJCf_o6HIbz8oKz5GqieUfM1VN2094BYok&usqp=CAU"
	case "testcafe":
		return "https://coursinator.com/blog/wp-content/uploads/testcafe-twitter-card-icon.png"
	case "puppeteer":
		return "https://cdn.dribbble.com/users/3800131/screenshots/15188869/media/823b8d9b8055e21c18408aca4342ae60.png"
	case "espresso":
		return "https://developer.android.com/images/training/testing/espresso.png"
	case "xcuitest":
		return "https://secureservercdn.net/198.71.233.197/a9j.9a0.myftpupload.com/wp-content/uploads/2019/01/XCUITest-framework.jpg"
	default:
		return ""
	}
}

func (s *Notifier) getTestEnvEmoji() string {
	if s.TestEnv == "sauce" {
		return ":saucy:"
	}
	return ":docker:"
}

func regenerateName(name, wholeName string, length int) string {
	minus := length - len(name)
	for minus > 0 {
		wholeName = fmt.Sprintf("%s%s", wholeName, " ")
		minus--
	}
	return wholeName
}

func (s *Notifier) getJobURL(name, jobURL string) string {
	return fmt.Sprintf("<%s|%s>", jobURL, name)
}

func (s *Notifier) creatAttachment() slack.Attachment {
	color := "#F00000"
	if s.Passed {
		color = "#008000"
	}
	return slack.Attachment{
		Color:  color,
		Blocks: slack.Blocks{BlockSet: s.createBlocks()},
	}
}

func statusSymbol(passed bool) string {
	if !passed {
		return "✖      "
	}

	return "✔      "
}

func statusText(passed bool) string {
	if !passed {
		return "failed"
	}
	return "passed"
}
