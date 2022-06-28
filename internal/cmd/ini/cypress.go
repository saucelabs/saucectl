package ini

import (
	// imports embed to load .sauceignore
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	v1 "github.com/saucelabs/saucectl/internal/cypress/v1"
	"github.com/saucelabs/saucectl/internal/cypress/v1alpha"
)

func configureCypress(cfg *initConfig) interface{} {
	versions := strings.Split(cfg.frameworkVersion, ".")
	version, err := strconv.Atoi(versions[0])
	if err != nil {
		log.Err(err).Msg("failed to parse frameworkVersion")
	}
	if version < 10 {
		return v1alpha.Project{
			TypeDef: config.TypeDef{
				APIVersion: v1alpha.APIVersion,
				Kind:       cypress.Kind,
			},
			Sauce: config.SauceConfig{
				Region:      cfg.region,
				Sauceignore: ".sauceignore",
				Concurrency: cfg.concurrency,
			},
			RootDir: ".",
			Cypress: v1alpha.Cypress{
				Version:    cfg.frameworkVersion,
				ConfigFile: cfg.cypressJSON,
			},
			Suites: []v1alpha.Suite{
				{
					Name:         fmt.Sprintf("cypress - %s - %s", firstNotEmpty(cfg.platformName, cfg.mode), cfg.browserName),
					PlatformName: cfg.platformName,
					Browser:      cfg.browserName,
					Mode:         cfg.mode,
					Config: v1alpha.SuiteConfig{
						TestFiles: []string{"**/*.*"},
					},
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

	return v1.Project{
		TypeDef: config.TypeDef{
			APIVersion: v1.APIVersion,
			Kind:       cypress.Kind,
		},
		Sauce: config.SauceConfig{
			Region:      cfg.region,
			Sauceignore: ".sauceignore",
			Concurrency: cfg.concurrency,
		},
		RootDir: ".",
		Cypress: v1.Cypress{
			Version:    cfg.frameworkVersion,
			ConfigFile: cfg.cypressJSON,
		},
		Suites: []v1.Suite{
			{
				Name:         fmt.Sprintf("cypress - %s - %s", firstNotEmpty(cfg.platformName, cfg.mode), cfg.browserName),
				PlatformName: cfg.platformName,
				Browser:      cfg.browserName,
				Mode:         cfg.mode,
				Config: v1.SuiteConfig{
					TestingType: "e2e",
					SpecPattern: []string{"**/*.*"},
				},
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

//go:embed sauceignore/cypress.sauceignore
var sauceignoreCypress string
