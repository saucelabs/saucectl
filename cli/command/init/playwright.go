package init

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/playwright"
)

func configurePlaywright(cfg *initConfig) interface{} {
	return playwright.Project{
		TypeDef: config.TypeDef{
			APIVersion: config.VersionV1Alpha,
			Kind:       config.KindPlaywright,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: 2, //TODO: Use MIN(AccountLimit, 10)
		},
		RootDir: ".",
		Playwright: playwright.Playwright{
			Version: cfg.frameworkVersion,
		},
		Suites: []playwright.Suite{
			{
				Name:         "My First Suite", //TODO: Authorize to name you suite
				PlatformName: cfg.platformName,
				Params: playwright.SuiteConfig{
					BrowserName: cfg.browserName,
				},
				Mode: cfg.mode,
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

func sauceignorePlaywright() string {
	return `# This file instructs saucectl to not package any files mentioned here.
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
# Remove this to have node_modules uploaded with code
node_modules/
`
}