package buildtable

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
)

// Reporter is an implementation of report.Reporter
// It wraps a table reporter and decorates it with additional metadata
type Reporter struct {
	TableReporter table.Reporter
}

// Add adds a TestResult to the report
func (r *Reporter) Add(t report.TestResult) {
	r.TableReporter.Add(t)
}

// Render renders the report
func (r *Reporter) Render() {
	printPadding(2)
	printTitle()
	printPadding(2)

	r.TableReporter.Render()

	printPadding(1)
	printBuildLink()
	printPadding(1)
}

// Reset resets the report
func (r *Reporter) Reset() {
	r.TableReporter.Reset()
}

// ArtifactRequirements returns a list of artifact types are this reporter requires to create a proper report.
func (r *Reporter) ArtifactRequirements() []report.ArtifactType {
	return r.TableReporter.ArtifactRequirements()
}

func printPadding(repeat int) {
	fmt.Print(strings.Repeat("\n", repeat))
}

func printTitle() {
	rl := color.New(color.FgBlue, color.Underline, color.Bold).Sprintf("Results:")
	fmt.Printf("  %s", rl)
}

func printBuildLink() {
	label := color.New(color.FgBlue).Sprint("Build Link:")
	link := color.New(color.Underline).Sprint("https://app.saucelabs.com/builds/vdc/0f7e482cc6873444b462cca19ef74a18")

	fmt.Printf("  %s %s", label, link)
}
