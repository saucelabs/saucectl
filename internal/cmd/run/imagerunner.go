package run

import (
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/table"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func runImageRunner(cmd *cobra.Command) (int, error) {
	config.ValidateSchema(gFlags.cfgFilePath)
	p, err := imagerunner.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	if err := applyImageRunnerFlags(&p); err != nil {
		return 1, err
	}
	imagerunner.SetDefaults(&p)

	if err := imagerunner.Validate(p); err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	imageRunnerClient.URL = regio.APIBaseURL()
	restoClient.URL = regio.APIBaseURL()

	tracker := segment.DefaultTracker

	go func() {
		props := usage.Properties{}
		props.SetFramework("imagerunner").SetFlags(cmd.Flags()).SetSauceConfig(p.Sauce).
			SetArtifacts(p.Artifacts).SetNumSuites(len(p.Suites))
		tracker.Collect(cases.Title(language.English).String(cmds.FullName(cmd)), props)
		_ = tracker.Close()
	}()

	r := saucecloud.ImgRunner{
		Project:       p,
		RunnerService: &imageRunnerClient,
		TunnelService: &restoClient,
		Reporters: []report.Reporter{&table.Reporter{
			Dst: os.Stdout,
		}},
	}
	return r.RunProject()
}

func applyImageRunnerFlags(p *imagerunner.Project) error {
	if gFlags.selectedSuite != "" {
		if err := imagerunner.FilterSuites(p, gFlags.selectedSuite); err != nil {
			return err
		}
	}
	return nil
}
