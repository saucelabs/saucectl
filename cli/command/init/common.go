package init

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"

	"github.com/saucelabs/saucectl/internal/config"
)


var configurators = map[string]func(cfg *initConfig) interface{}{
	"cypress":    configureCypress,
	"espresso":   configureEspresso,
	"playwright": configurePlaywright,
	"puppeteer":  configurePuppeteer,
	"testcafe":   configureTestcafe,
	"xcuitest":   configureXCUITest,
}

var sauceignoreGenerators = map[string]func() string {
	"cypress":    sauceignoreCypress,
	"playwright": sauceignorePlaywright,
	"puppeteer":  sauceignorePuppeteer,
	"testcafe":   sauceignoreTestcafe,
}

func saveConfigurationFiles(initCfg *initConfig) ([]string, error) {
	var files []string

	configFormatter, ok := configurators[initCfg.frameworkName]
	if ok {
		err := saveSauceConfig(configFormatter(initCfg))
		if err != nil {
			return []string{}, err
		}
		files = append(files, ".sauce/config.yml")
	}

	sauceignoreGenerator, ok := sauceignoreGenerators[initCfg.frameworkName]
	if ok {
		err := saveSauceIgnore(sauceignoreGenerator())
		if err != nil {
			return []string{}, err
		}
		files = append(files, ".sauceignore")
	}
	return files, nil
}

func saveSauceConfig(p interface{}) error {
	fi, err := os.Stat(".sauce")
	if err != nil && os.IsNotExist(err) {
		if err = os.Mkdir(".sauce", 0750); err != nil {
			return err
		}
		fi, err = os.Stat(".sauce")
		if err != nil {
			return err
		}
	}
	if !fi.IsDir() {
		return fmt.Errorf(".sauce exists and is not a directory")
	}
	fd, err := os.Create(".sauce/config.yml")
	if err != nil {
		return err
	}
	defer fd.Close()
	if err = yaml.NewEncoder(fd).Encode(p); err != nil {
		return err
	}
	return nil
}

func saveSauceIgnore(content string) error {
	fd, err := os.Create(".sauceignore")
	if err != nil {
		return err
	}
	defer fd.Close()
	fd.WriteString(content)
	return nil
}

func displaySummary(files []string) {
	println()
	log.Info().Msg("The following files have been created:")
	for _, f := range files {
		log.Info().Msgf("  %s", f)
	}
	println()
}

func completeBasic(toComplete string) []string {
	files, _ := filepath.Glob(fmt.Sprintf("%s%s", toComplete, "*"))
	return files
}

func extValidator(framework string) survey.Validator {
	var exts []string
	switch framework {
	case config.KindEspresso:
		exts = []string{".apk"}
	case config.KindXcuitest:
		exts = []string{".ipa", ".app"}
	case config.KindCypress:
		exts = []string{".json"}
	}

	return func(s interface{}) error {
		val := s.(string)
		found := false
		for _, ext := range exts {
			if strings.HasSuffix(val, ext) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("invalid extension. must be one of the following: %s", strings.Join(exts, ", "))
		}
		_, err := os.Stat(val)
		if err != nil {
			return fmt.Errorf("%s: %v", val, err)
		}
		return nil
	}
}

func uniqSorted(ss []string) []string {
	var out []string
	idx := make(map[string]bool)
	for _, s := range ss {
		if _, ok := idx[s]; !ok {
			idx[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}