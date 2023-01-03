package run

import (
	"github.com/saucelabs/saucectl/internal/htexec"
	// "github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/saucecloud"
	"github.com/spf13/cobra"
)

func NewHtexecCmd() *cobra.Command {
	return nil
}

func runHTExec(cmd *cobra.Command) (int, error) {
	p, err := htexec.FromFile(gFlags.cfgFilePath)
	if err != nil {
		return 1, err
	}

	// regio := region.FromString(p.Sauce.Region)
	// htexecClient.URL = regio.APIBaseURL()
	htexecClient.URL = "http://127.0.0.1:4010"

	r := saucecloud.HostedRunner{
		Project: p,
		RunnerService: &htexecClient,
	}
	return r.Run()
}
