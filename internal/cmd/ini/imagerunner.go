package ini

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/imagerunner"
)

func configureImageRunner(cfg *initConfig) interface{} {
	return imagerunner.Project{
		TypeDef: config.TypeDef{
			APIVersion: imagerunner.APIVersion,
			Kind:       imagerunner.Kind,
		},
		Sauce: config.SauceConfig{
			Region: cfg.region,
		},
		Suites: []imagerunner.Suite{
			{
				Name:  fmt.Sprintf("imagerunner - %s", cfg.dockerImage),
				Image: cfg.dockerImage,
				ImagePullAuth: imagerunner.ImagePullAuth{
					User:  "${DOCKER_USERNAME}",
					Token: "${DOCKER_PASSWORD}",
				},
				Workload: cfg.workload,
			},
		},
		Artifacts: config.Artifacts{
			Download: config.ArtifactDownload{
				When:      cfg.artifactWhen,
				Directory: "./artifacts",
				Match:     []string{"*"},
			},
		},
	}
}

func displayExtraInfoImageRunner() {
	println()
	color.HiGreen("Before running your tests, you need to set the following environment variables:")
	color.Green("  - DOCKER_USERNAME")
	color.Green("  - DOCKER_PASSWORD")
	println()
}
