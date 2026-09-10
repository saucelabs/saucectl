package run

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/http"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/saucelabs/saucectl/internal/usage"
)

// runAuthoring is the `saucectl run` path for `kind: authoring`: AI-authored
// test cases run through the shared reporters, artifact download,
// concurrency control and exit codes like every other kind.
func runAuthoring(cmd *cobra.Command, isCLIDriven bool) (int, error) {
	if !isCLIDriven {
		config.ValidateSchema(gFlags.cfgFilePath)
	}

	p, err := authoring.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}
	if gFlags.selectedSuite != "" {
		if err := authoring.FilterSuites(&p, gFlags.selectedSuite); err != nil {
			return 1, err
		}
	}
	authoring.SetDefaults(&p)
	if err := authoring.Validate(p); err != nil {
		return 1, err
	}
	if gFlags.failFast {
		log.Warn().Msg("--fail-fast is not supported for kind: authoring and will be ignored.")
	}

	regio := region.FromString(p.Sauce.Region)
	creds := regio.Credentials()

	svc := http.NewAuthoringService(regio, creds, authoringTimeout)
	iamClient := http.NewUserService(regio.APIBaseURL(), creds, iamTimeout)
	if _, err := authoring.VerifyEntitlement(cmd.Context(), &iamClient, &svc); err != nil {
		return 1, err
	}

	restoClient := http.NewResto(regio, creds.Username, creds.AccessKey, 0)
	rdcClient := http.NewRDCService(regio, creds.Username, creds.AccessKey, rdcTimeout)
	jobService := saucecloud.JobService{
		RDC:                    rdcClient,
		Resto:                  restoClient,
		ArtifactDownloadConfig: p.Artifacts.Download,
	}
	buildService := http.NewBuildService(regio, creds.Username, creds.AccessKey, buildTimeout)

	tracker := usage.DefaultClient
	if regio == region.Staging {
		tracker.Enabled = false
	}
	go func() {
		tracker.Collect(
			cmds.FullName(cmd),
			usage.Framework("authoring", ""),
			usage.Flags(cmd.Flags()),
			usage.SauceConfig(p.Sauce),
			usage.Artifacts(p.Artifacts),
			usage.NumSuites(len(p.Suites)),
			usage.Reporters(p.Reporters),
		)
		_ = tracker.Close()
	}()

	cleanupArtifacts(p.Artifacts)

	return runAuthoringInCloud(cmd.Context(), p, regio, &svc, jobService, &buildService, &restoClient)
}

// runAuthoringInCloud wires the runner and executes it.
func runAuthoringInCloud(ctx context.Context, p authoring.Project, regio region.Region, svc *http.AuthoringService, jobService saucecloud.JobService, buildService *http.BuildService, restoClient *http.Resto) (int, error) {
	log.Info().
		Str("region", regio.String()).
		Str("tunnel", p.Sauce.Tunnel.Name).
		Str("build", p.Sauce.Metadata.Build).
		Msg("Running AI-authored tests in Sauce Labs.")

	r := authoring.Runner{
		Project:    p,
		TestCases:  svc,
		TestSuites: svc,
		Artifacts:  jobService,
		Stopper:    jobService,
		Builds:     buildService,
		Tunnels:    restoClient,
		Region:     regio,
		Reporters:  createReporters(p.Reporters, gFlags.async),
		Async:      gFlags.async,
	}

	exitCode, err := r.RunProject(ctx)
	if err != nil {
		return exitCode, fmt.Errorf("running AI-authored tests: %w", err)
	}
	return exitCode, nil
}
