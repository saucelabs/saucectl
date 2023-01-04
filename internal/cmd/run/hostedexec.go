package run

import (
	"github.com/saucelabs/saucectl/internal/hostedexec"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/spf13/cobra"
)

func NewHtexecCmd() *cobra.Command {
	return nil
}

func runHostedExec(cmd *cobra.Command) (int, error) {
	p, err := hostedexec.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	regio := region.FromString(p.Sauce.Region)
	hostedExecClient.URL = regio.APIBaseURL()
	// hostedExecClient.URL = "http://127.0.0.1:4010"

	r := saucecloud.HostedExecRunner{
		Project: p,
		RunnerService: &hostedExecClient,
	}
	return r.Run()
}
