package slack

import (
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
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
			slack.MsgOptionText("sauceCTL test result", false),
			slack.MsgOptionAttachments(attachment),
			slack.MsgOptionAsUser(true),
		)
		if err != nil {
			log.Error().Msgf("Failed to send message to slack, err: %s", err.Error())
		}
		log.Info().Msgf("Message successfully sent to slack channel %s at %s", channelID, timestamp)

	}
}

func (s *SlackNotifier) newMsg() slack.Attachment {
	color := "F00000"
	if s.Passed {
		color = "#008000"
	}
	return slack.Attachment{
		Color:  color,
		Title:  s.TestName,
		Blocks: s.createBlocks(),
	}
}

func (s *SlackNotifier) createBlocks() slack.Blocks {
	header := slack.NewHeaderBlock(slack.NewTextBlockObject("header", "All tests passed", true, true))
	tableHeader := []*slack.TextBlockObject{}
	tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", "*Date*", true, true))
	tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", "*Framework*", true, true))
	tableHeader = append(tableHeader, slack.NewTextBlockObject("plain_text", time.Now().String(), true, true))
	tableHeader = append(tableHeader, slack.NewTextBlockObject("plain_text", s.Framework, true, true))

	tableHeadSection := slack.NewSectionBlock(slack.NewTextBlockObject("test", "test", true, true), tableHeader, nil)

	blocks := []slack.Block{}
	blocks = append(blocks, header, tableHeadSection)
	fmt.Println("===========")
	fmt.Println("===========")
	fmt.Println("===========")
	fmt.Println("===========")
	spew.Dump(blocks)
	fmt.Println("===========")
	fmt.Println("===========")
	fmt.Println("===========")

	return slack.Blocks{BlockSet: blocks}
}

func (s *SlackNotifier) genTemplate() error {

	return nil
}
