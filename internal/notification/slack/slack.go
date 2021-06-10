package slack

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
)

// SlackNotifier represents notifier for slack
type SlackNotifier struct {
	Token       string
	Channels    []string
	TestResults []TestResult
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

const (
	SendNever     = "never"
	SendAlways    = "always"
	SendOnFailure = "failure"
)

func (s *SlackNotifier) SendMessage() {
	api := slack.New(s.Token)
	attachment := s.newMsg()

	for _, c := range s.Channels {
		channelID, timestamp, err := api.PostMessage(
			c,
			slack.MsgOptionText("saucectl result", false),
			slack.MsgOptionAttachments(attachment),
			slack.MsgOptionAsUser(true),
		)
		if err != nil {
			log.Error().Msg("Failed to send message to slack")
		}
		log.Info().Msgf("Message successfully sent to slack channel %s at %s", channelID, timestamp)

	}
}

func (s *SlackNotifier) newMsg() slack.Attachment {
	return slack.Attachment{
		Title:  "tian is testing...",
		Blocks: s.createBlocks(),
	}
}

func (s *SlackNotifier) createBlocks() slack.Blocks {
	blocks := slack.Blocks{}
	headerText := slack.NewTextBlockObject("mrkdwn", "You have a new request:\n*<fakeLink.toEmployeeProfile.com|Fred Enriquez - New device request>*", false, false)
	blocks.BlockSet = append(blocks.BlockSet, headerText)

	return blocks
}
