package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/puppeteer"
)

func configurePuppeteer(cfg *initConfig) interface{} {
	return puppeteer.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindPuppeteer,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: ".",
		Puppeteer: puppeteer.Puppeteer{
			Version: cfg.frameworkVersion,
		},
		Suites: []puppeteer.Suite{
			{
				Name:    "My First Suite", //TODO: Authorize to name you suite
				Browser: cfg.browserName,
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

func sauceignorePuppeteer() string {
	return `# This file instructs saucectl to not package any files mentioned here.
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