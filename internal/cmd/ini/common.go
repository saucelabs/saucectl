package ini

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/xcuitest"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"gopkg.in/yaml.v2"
)

var configurators = map[string]func(cfg *initConfig) interface{}{
	"cypress":    configureCypress,
	"espresso":   configureEspresso,
	"playwright": configurePlaywright,
	"testcafe":   configureTestcafe,
	"xcuitest":   configureXCUITest,
	"xctest":     configureXCTest,
}

var sauceignores = map[string]string{
	"cypress":    sauceignoreCypress,
	"playwright": sauceignorePlaywright,
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

	sauceignore, ok := sauceignores[initCfg.frameworkName]
	if ok {
		err := saveSauceIgnore(sauceignore)
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
	return yaml.NewEncoder(fd).Encode(p)
}

func saveSauceIgnore(content string) error {
	fd, err := os.Create(".sauceignore")
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = fd.WriteString(content)
	return err
}

func displaySummary(files []string) {
	fmt.Println()
	color.HiGreen("The following files have been created:")
	for _, f := range files {
		color.Green("  %s", f)
	}
	fmt.Println()
}

func completeBasic(toComplete string) []string {
	files, _ := filepath.Glob(fmt.Sprintf("%s%s", toComplete, "*"))
	return files
}

func frameworkExtValidator(framework, frameworkVersion string) survey.Validator {
	var exts []string
	switch framework {
	case espresso.Kind:
		exts = []string{".apk", ".aab"}
	case xcuitest.Kind:
		exts = []string{".ipa", ".app"}
	case cypress.Kind:
		exts = []string{".js", ".ts", ".mjs", ".cjs"}
		if getMajorVersion(frameworkVersion) < 10 {
			exts = []string{".json"}
		}
	}

	return extValidator(exts)
}

func extValidator(validExts []string) survey.Validator {
	return func(s interface{}) error {
		val := s.(string)
		found := false
		for _, ext := range validExts {
			if strings.HasSuffix(val, ext) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("invalid extension. must be one of the following: %s", strings.Join(validExts, ", "))
		}
		_, err := os.Stat(val)
		if err != nil {
			return fmt.Errorf("%s: %v", val, err)
		}
		return nil
	}
}

func dockerImageValidator() survey.Validator {
	re := regexp.MustCompile(`^([\w.\-_]+((:\d+|)(/[a-z0-9._-]+/[a-z0-9._-]+))|)(/|)([a-z0-9.\-_]+(/[a-z0-9.\-_]+|))(:([\w.\-_]{1,127})|)$`)
	return func(s interface{}) error {
		str := s.(string)
		if re.MatchString(str) {
			return nil
		}
		return fmt.Errorf("%s is not a valid docker image", str)
	}
}

func getMajorVersion(frameworkVersion string) int {
	version := strings.Split(frameworkVersion, ".")[0]
	v, err := strconv.Atoi(version)
	if err != nil {
		log.Err(err).Msg("failed to get framework version.")
		return 0
	}
	return v
}

func sortVersions(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		v1 := strings.Split(versions[i], ".")
		v2 := strings.Split(versions[j], ".")
		v1Major, _ := strconv.Atoi(v1[0])
		v2Major, _ := strconv.Atoi(v2[0])

		if v1Major == v2Major && len(v1) > 1 && len(v2) > 1 {
			return strings.Compare(v1[1], v2[1]) == 1
		}
		return v1Major > v2Major
	})
}

func sliceContainsString(slice []string, val string) bool {
	for _, value := range slice {
		if value == val {
			return true
		}
	}
	return false
}
