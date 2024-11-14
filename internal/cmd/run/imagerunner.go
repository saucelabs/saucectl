package run

import (
	"os"

	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/report/json"
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
	tracker := segment.DefaultTracker
	if regio == region.Staging {
		tracker.Enabled = false
	}

	go func() {
		tracker.Collect(
			cases.Title(language.English).String(cmds.FullName(cmd)),
			usage.Framework("imagerunner", ""),
			usage.Flags(cmd.Flags()),
			usage.SauceConfig(p.Sauce),
			usage.Artifacts(p.Artifacts),
			usage.NumSuites(len(p.Suites)),
		)
		_ = tracker.Close()
	}()

	reporters := []report.Reporter{
		&table.Reporter{
			Dst: os.Stdout,
		},
	}
	if !gFlags.async {
		if p.Reporters.JSON.Enabled {
			reporters = append(reporters, &json.Reporter{
				WebhookURL: p.Reporters.JSON.WebhookURL,
				Filename:   p.Reporters.JSON.Filename,
			})
		}
	}

	cleanupArtifacts(p.Artifacts)

	creds := regio.Credentials()

	asyncEventManager, err := http.NewAsyncEventMgr()
	if err != nil {
		return 1, err
	}

	imageRunnerClient := http.NewImageRunner(regio.APIBaseURL(), creds, imgExecTimeout, asyncEventManager)
	restoClient := http.NewResto(regio, creds.Username, creds.AccessKey, 0)

	r := saucecloud.NewImgRunner(p, &imageRunnerClient, &restoClient, asyncEventManager,
		reporters, gFlags.async)

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
