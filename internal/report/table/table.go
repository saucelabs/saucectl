package table

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/saucelabs/saucectl/internal/report"
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
		MiddleHorizontal: "─",
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

// Reporter is a table writer implementation for report.Reporter.
type Reporter struct {
	TestResults []report.TestResult
	Dst         io.Writer
	lock        sync.Mutex
}

// Add adds the test result to the summary table.
func (r *Reporter) Add(t report.TestResult) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.TestResults = append(r.TestResults, t)
}

// Render renders out a test summary table to the destination of Reporter.Dst.
func (r *Reporter) Render() {
	r.lock.Lock()
	defer r.lock.Unlock()

	t := table.NewWriter()
	t.SetOutputMirror(r.Dst)
	t.SetStyle(defaultTableStyle)
	t.SuppressEmptyColumns()

	t.AppendHeader(table.Row{"", "Name", "Duration", "Status", "Browser", "Platform", "Device"})
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Number:   0, // it's the first nameless column that contains the passed/fail icon
			WidthMax: 1,
		},
		{
			Name:     "Name",
			WidthMin: 30,
		},
		{
			Name:        "Duration",
			Align:       text.AlignRight,
			AlignFooter: text.AlignRight,
		},
	})

	errors := 0
	var totalDur time.Duration
	for _, ts := range r.TestResults {
		if !ts.Passed {
			errors++
		}

		totalDur += ts.Duration

		// the order of values must match the order of the header
		t.AppendRow(table.Row{statusSymbol(ts.Passed), ts.Name, ts.Duration.Truncate(1 * time.Second),
			statusText(ts.Passed), ts.Browser, ts.Platform, ts.DeviceName})
	}

	t.AppendFooter(footer(errors, len(r.TestResults), totalDur))

	_, _ = fmt.Fprintln(r.Dst)
	t.Render()
}

// Reset resets the reporter to its initial state. This action will delete all test results.
func (r *Reporter) Reset() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.TestResults = make([]report.TestResult, 0)
}

// ArtifactRequirements returns a list of artifact types are this reporter requires to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return nil
}

func footer(errors, tests int, dur time.Duration) table.Row {
	symbol := statusSymbol(errors == 0)
	if errors != 0 {
		relative := float64(errors) / float64(tests) * 100
		return table.Row{symbol, fmt.Sprintf("%d of %d suites have failed (%.0f%%)", errors, tests, relative), dur.Truncate(1 * time.Second)}
	}
	return table.Row{symbol, "All tests have passed", dur.Truncate(1 * time.Second)}
}

func statusText(passed bool) string {
	if !passed {
		return color.RedString("failed")
	}

	return color.GreenString("passed")
}

func statusSymbol(passed bool) string {
	if !passed {
		return color.RedString("✖")
	}

	return color.GreenString("✔")
}
