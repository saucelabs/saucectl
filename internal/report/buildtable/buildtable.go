package buildtable

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/build"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
)

// Reporter is an implementation of report.Reporter
// It wraps a table reporter and decorates it with additional metadata
type Reporter struct {
	Service        build.Reader
	VDCTableReport table.Reporter
	RDCTableReport table.Reporter
}

func New(svc build.Reader) Reporter {
	return Reporter{
		Service: svc,
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

	var jURL string
	var bURL string
	if len(r.VDCTableReport.TestResults) > 0 {
		r.VDCTableReport.Render()

		for _, tr := range r.VDCTableReport.TestResults {
			if tr.URL != "" {
				jURL = tr.URL
				break
			}
		}
		bURL = r.buildURLFromJobURL(jURL, build.VDC)

		if bURL == "" {
			bURL = "N/A"
		}
		printPadding(1)
		printBuildLink(bURL)
		printPadding(2)
	}
	if len(r.RDCTableReport.TestResults) > 0 {
		r.RDCTableReport.Render()

		for _, tr := range r.RDCTableReport.TestResults {
			if tr.URL != "" {
				jURL = tr.URL
				break
			}
		}
		bURL = r.buildURLFromJobURL(jURL, build.RDC)

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

func (r *Reporter) buildURLFromJobURL(jobURL string, buildSource build.Source) string {
	pURL, err := url.Parse(jobURL)
	if err != nil {
		log.Debug().Err(err).Msgf("Failed to parse job url (%s)", jobURL)
		return ""
	}
	p := strings.Split(pURL.Path, "/")
	jID := p[len(p)-1]

	bID, err := r.Service.GetBuildID(context.Background(), jID, buildSource)
	if err != nil {
		log.Debug().Err(err).Msgf("Failed to retrieve build id for job (%s)", jID)
		return ""
	}

	return fmt.Sprintf("%s://%s/builds/%s/%s", pURL.Scheme, pURL.Host, buildSource, bID)
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
