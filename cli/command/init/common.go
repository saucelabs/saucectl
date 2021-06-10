package init

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"

	"github.com/saucelabs/saucectl/internal/config"
)

func saveConfiguration(p interface{}) error {
	fi, err := os.Stat(".sauce")
	if err != nil && os.IsNotExist(err) {
		if err = os.Mkdir(".sauce", 0750); err != nil {
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

func isDirectory(s interface{}) error {
	val := s.(string)
	fi, err := os.Stat(val)
	if err != nil {
		return fmt.Errorf("%s: %v", val, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", val)
	}
	return nil
}