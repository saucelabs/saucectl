package buildtable

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
)

// Reporter is an implementation of report.Reporter
// It wraps a table reporter and decorates it with additional metadata
type Reporter struct {
	VDCTableReport table.Reporter
	RDCTableReport table.Reporter
}

func New() Reporter {
	return Reporter{
		VDCTableReport: table.Reporter{
			Dst: os.Stdout,
		},
		RDCTableReport: table.Reporter{
			Dst: os.Stdout,
		},
	}
}

// Add adds a TestResult to the report
func (r *Reporter) Add(t report.TestResult) {
	if t.RDC {
		r.RDCTableReport.Add(t)
	} else {
		r.VDCTableReport.Add(t)
	}
}

// Render renders the report
func (r *Reporter) Render() {
	printPadding(2)
	printTitle()
	printPadding(2)

	if len(r.VDCTableReport.TestResults) > 0 {
		r.VDCTableReport.Render()

		var bURL string
		for _, result := range r.VDCTableReport.TestResults {
			if result.BuildURL != "" {
				bURL = result.BuildURL
				break
			}
		}

		if bURL == "" {
			bURL = "N/A"
		}
		printPadding(1)
		printBuildLink(bURL)
		printPadding(2)
	}
	if len(r.RDCTableReport.TestResults) > 0 {
		r.RDCTableReport.Render()

		var bURL string
		for _, result := range r.RDCTableReport.TestResults {
			if result.BuildURL != "" {
				bURL = result.BuildURL
				break
			}
		}

		if bURL == "" {
			bURL = "N/A"
		}
		printPadding(1)
		printBuildLink(bURL)
		printPadding(2)
	}
}

// Reset resets the report
func (r *Reporter) Reset() {
	r.VDCTableReport.Reset()
	r.RDCTableReport.Reset()
}

// ArtifactRequirements returns a list of artifact types are this reporter requires to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return nil
}

func printPadding(repeat int) {
	fmt.Print(strings.Repeat("\n", repeat))
}

func printTitle() {
	rl := color.New(color.FgBlue, color.Underline, color.Bold).Sprintf("Results:")
	fmt.Printf("  %s", rl)
}

func printBuildLink(buildURL string) {
	label := color.New(color.FgBlue).Sprint("Build Link:")
	link := color.New(color.Underline).Sprint(buildURL)

	fmt.Printf("  %s %s", label, link)
}
