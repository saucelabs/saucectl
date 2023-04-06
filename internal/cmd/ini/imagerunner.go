package ini

import (
	"fmt"
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
