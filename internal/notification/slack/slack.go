package slack

import (
	"time"

	"github.com/saucelabs/saucectl/internal/config"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
)

// SlackNotifier represents notifier for slack
type SlackNotifier struct {
	Token       string
	Channels    []string
	TestResults []TestResult
	Framework   string
	Passed      bool
	TestName    string
}

type TestResult struct {
	Name       string
	Duration   time.Duration
	Passed     bool
	Browser    string
	Platform   string
	DeviceName string
	JobURL     string
}

func (s *SlackNotifier) SendMessage() {
	api := slack.New(s.Token)
	//attachment := s.newMsg()

	for _, c := range s.Channels {
		channelID, timestamp, err := api.PostMessage(
			c,
			slack.MsgOptionText("sauceCTL test result", false),
			slack.MsgOptionBlocks(s.createBlocks()...),
			slack.MsgOptionAsUser(true),
		)
		if err != nil {
			log.Error().Msgf("Failed to send message to slack, err: %s", err.Error())
		}
		log.Info().Msgf("Message successfully sent to slack channel %s at %s", channelID, timestamp)

	}
}

func (s *SlackNotifier) createBlocks() []slack.Block {
	headerText := slack.NewTextBlockObject("mrkdwn", "All tests passed", false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	tableHeader := []*slack.TextBlockObject{}
	tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", "*Date*", false, false))
	tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", "*Framework*", false, false))
	tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", time.Now().String(), false, false))
	tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", s.Framework, false, false))

	tableHeadSection := slack.NewSectionBlock(nil, tableHeader, nil)

	blocks := make([]slack.Block, 0)
	blocks = append(blocks, headerSection)
	blocks = append(blocks, tableHeadSection)
	return blocks
	//fmt.Println("===========")
	//fmt.Println("===========")
	//fmt.Println("===========")
	//fmt.Println("===========")
	//spew.Dump(blocks)
	//fmt.Println("===========")
	//fmt.Println("===========")
	//fmt.Println("===========")
	//
	//return slack.Blocks{BlockSet: blocks}
}

func (s *SlackNotifier) genTemplate() error {

	return nil
}

// ShouldSendNotification returns true if it should send notification, otherwise false
func ShouldSendNotification(jobID string, passed bool, cfg config.Notifications) bool {
	if jobID == "" {
		return false
	}

	if cfg.Slack.Token == "" {
		return false
	}

	if len(cfg.Slack.Channels) == 0 {
		return false
	}

	if cfg.Slack.Send == config.SendNever {
		return false
	}

	if cfg.Slack.Send == config.SendAlways {
		return true
	}

	if cfg.Slack.Send == config.SendOnFailure && !passed {
		return true
	}

	return false
}
