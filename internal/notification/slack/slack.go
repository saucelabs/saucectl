package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/credentials"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/slack-go/slack"
)

var defaultTableStyle = table.Style{
	Name: "saucy",
	Box: table.BoxStyle{
		BottomLeft:       "└",
		BottomRight:      "┘",
		BottomSeparator:  "",
		EmptySeparator:   text.RepeatAndTrim(" ", text.RuneCount("+")),
		Left:             "│",
		LeftSeparator:    "",
		MiddleHorizontal: " ",
		MiddleSeparator:  "",
		MiddleVertical:   "",
		PaddingLeft:      "  ",
		PaddingRight:     "  ",
		PageSeparator:    "\n",
		Right:            "│",
		RightSeparator:   "",
		TopLeft:          "┌",
		TopRight:         "┐",
		TopSeparator:     "",
		UnfinishedRow:    " ...",
	},
	Color: table.ColorOptionsDefault,
	Format: table.FormatOptions{
		Footer: text.FormatDefault,
		Header: text.FormatDefault,
		Row:    text.FormatDefault,
	},
	HTML: table.DefaultHTMLOptions,
	Options: table.Options{
		DrawBorder:      false,
		SeparateColumns: false,
		SeparateFooter:  true,
		SeparateHeader:  true,
		SeparateRows:    false,
	},
	Title: table.TitleOptionsDefault,
}

// SlackNotifier represents notifier for slack
type SlackNotifier struct {
	Token       string
	Channels    []string
	TestResults []TestResult
	Framework   string
	Passed      bool
	Region      region.Region
	Metadata    config.Metadata
	TestEnv     string
}

type TestResult struct {
	Name       string
	Duration   time.Duration
	Passed     bool
	Browser    string
	Platform   string
	DeviceName string
	JobID      string
	JobURL     string
}

func (s *SlackNotifier) SendMessage() {
	api := slack.New(s.Token)
	//attachment := s.newMsg()

	for _, c := range s.Channels {
		channelID, timestamp, err := api.PostMessage(
			c,
			slack.MsgOptionText("sauceCTL test result", false),
			slack.MsgOptionAttachments(s.creatAttachment()),
			//slack.MsgOptionBlocks(s.createBlocks()...),
			slack.MsgOptionAsUser(true),
		)
		if err != nil {
			log.Error().Msgf("Failed to send message to slack, err: %s", err.Error())
		}
		log.Info().Msgf("Message successfully sent to slack channel %s at %s", channelID, timestamp)

	}
}

func (s *SlackNotifier) createBlocks() []slack.Block {
	headerText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s* %s", s.Metadata.Name, statusEmoji(s.Passed)), false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	contextElementText := slack.NewImageBlockElement(s.getFrameworkIcon(), "Framework icon")
	contextText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s %s | *Build ID*: %s | %s", s.getTestEnvEmoji(), credentials.Get().Username, s.Metadata.Build, time.Now().Format("2006-01-02 15:04:05")), false, false)
	frameworkIconSection := slack.NewContextBlock("", []slack.MixedElement{contextElementText, contextText}...)

	/*
		tableHeader := []*slack.TextBlockObject{}
		tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", "*Date*", false, false))
		tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", "*Author*", false, false))
		tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", time.Now().Format("2006-01-02 15:04:05"), false, false))
		tableHeader = append(tableHeader, slack.NewTextBlockObject("mrkdwn", credentials.Get().Username, false, false))

		tableHeadSection := slack.NewSectionBlock(nil, tableHeader, nil)
	*/

	resultText := slack.NewTextBlockObject("mrkdwn", s.RenderTable(), false, false)
	resultSection := slack.NewSectionBlock(resultText, nil, nil)

	blocks := make([]slack.Block, 0)
	blocks = append(blocks, headerSection)
	blocks = append(blocks, frameworkIconSection)
	//blocks = append(blocks, tableHeadSection)
	blocks = append(blocks, resultSection)

	return blocks
}

func (s *SlackNotifier) getFrameworkIcon() string {
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

func (s *SlackNotifier) getTestEnvEmoji() string {
	if s.TestEnv == "sauce" {
		return ":saucy:"
	}
	return ":docker:"
}

// ShouldSendNotification returns true if it should send notification, otherwise false
func (s *SlackNotifier) ShouldSendNotification(cfg config.Notifications) bool {
	for _, ts := range s.TestResults {
		if ts.JobID == "" {
			return false
		}
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

	if cfg.Slack.Send == config.SendOnFailure && !s.Passed {
		return true
	}

	return false
}

func regenerateName(name, wholeName string, length int) string {
	minus := length - len(name)
	for minus > 0 {
		wholeName = fmt.Sprintf("%s%s", wholeName, " ")
		minus--
	}
	return wholeName
}

func (s *SlackNotifier) RenderTable() string {
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
		tables = append(tables, []string{statusSymbol(ts.Passed), regenerateName(ts.Name, s.getJobURL(ts.Name, ts.JobID, ts.JobURL), longestName), ts.Duration.Truncate(1 * time.Second).String(),
			statusText(ts.Passed), ts.Browser, ts.Platform, ts.DeviceName})
	}
	var info string
	for _, t := range tables {
		info = fmt.Sprintf("%s\n%s", info, strings.Join(t, "\t"))
	}
	return info
	/*

		t := table.NewWriter()
		t.SetStyle(defaultTableStyle)
		t.SuppressEmptyColumns()

		t.AppendHeader(table.Row{"Passed", "Name", "Duration", "Status", "Browser", "Platform", "Device"})
		t.SetColumnConfigs([]table.ColumnConfig{
			{
				Number:   0, // it's the first nameless column that contains the passed/fail icon
				WidthMax: 1,
			},
			{
				Number:   1,
				WidthMax: 20,
			},
			{
				Name:        "Duration",
				Align:       text.AlignRight,
				AlignFooter: text.AlignRight,
			},
		})

		errors := 0
		var totalDur time.Duration
		for _, ts := range s.TestResults {
			if !ts.Passed {
				errors++
			}

			totalDur += ts.Duration

			// the order of values must match the order of the header
			t.AppendRow(table.Row{statusSymbol(ts.Passed), s.getJobURL(ts.Name, ts.JobID, ts.JobURL), ts.Duration.Truncate(1 * time.Second),
				statusText(ts.Passed), ts.Browser, ts.Platform, ts.DeviceName})
		}

		t.AppendFooter(footer(errors, len(s.TestResults), totalDur))

		return t.Render()
		return "`" + "`" + "`" + "\n" + t.Render() + "\n`" + "`" + "`"
	*/
}

func (s *SlackNotifier) getJobURL(name, ID, jobURL string) string {
	url := fmt.Sprintf("%s/tests/%s", s.Region.AppBaseURL(), ID)
	if jobURL != "" {
		url = jobURL
	}
	return fmt.Sprintf("<%s|%s>", url, name)
}

func (s *SlackNotifier) creatAttachment() slack.Attachment {
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

func statusEmoji(passed bool) string {
	if passed {
		return ":happy:"
	}
	return ":frogonfire:"
}

func statusText(passed bool) string {
	if !passed {
		return "failed"
	}

	return "passed"
}

func footer(errors, tests int, dur time.Duration) table.Row {
	symbol := statusSymbol(errors == 0)
	if errors != 0 {
		relative := float64(errors) / float64(tests) * 100
		return table.Row{symbol, fmt.Sprintf("%d of %d suites have failed (%.0f%%)", errors, tests, relative), dur.Truncate(1 * time.Second)}
	}
	return table.Row{symbol, "All tests have passed", dur.Truncate(1 * time.Second)}
}
