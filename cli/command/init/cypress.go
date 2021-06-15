package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
)

func configureCypress(cfg *initConfig) interface{} {
	return cypress.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindCypress,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: ".",
		Cypress: cypress.Cypress{
			Version:    cfg.frameworkVersion,
			ConfigFile: cfg.cypressJson,
		},
		Suites: []cypress.Suite{
			{
				Name:         "My First Suite", //TODO: Authorize to name you suite
				PlatformName: cfg.platformName,
				Browser:      cfg.browserName,
				Mode:         cfg.mode,
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

func sauceignoreCypress() string {
	return `
# This file instructs saucectl to not package any files mentioned here.
/examples/
/artifacts/
cypress/videos/
cypress/results/
cypress/screenshots/
# Remove this to have node_modules uploaded with code
node_modules/
.git/
.github/
.DS_Store
.hg/
.vscode/
.idea/
.gitignore
.hgignore
.gitlab-ci.yml
.npmrc
*.gif
`
}