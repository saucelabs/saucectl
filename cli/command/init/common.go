package init

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"strings"
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

func isJSON(s interface{}) error {
	val := s.(string)
	if !strings.HasSuffix(val, ".json") {
		return fmt.Errorf("configuration must be a .json")
	}
	_, err := os.Stat(val)
	if err != nil {
		return fmt.Errorf("%s: %v", val, err)
	}
	return nil
}

func completeJSON(toComplete string) []string {
	files, _ := filepath.Glob(fmt.Sprintf("%s%s", toComplete, "*"))
	return files
}

func isAnAPK(s interface{}) error {
	val := s.(string)
	if !strings.HasSuffix(val, ".apk") {
		return fmt.Errorf("application must be an .apk")
	}
	_, err := os.Stat(val)
	if err != nil {
		return fmt.Errorf("%s: %v", val, err)
	}
	return nil
}

func completeAPK(toComplete string) []string {
	files, _ := filepath.Glob(fmt.Sprintf("%s%s", toComplete, "*"))
	return files
}

func isAnIPAOrApp(s interface{}) error {
	val := s.(string)
	if !strings.HasSuffix(val, ".ipa") && !strings.HasSuffix(val, ".app") {
		return fmt.Errorf("application must be an .ipa or .apk")
	}
	_, err := os.Stat(val)
	if err != nil {
		return fmt.Errorf("%s: %v", val, err)
	}
	return nil
}

func completeIPA(toComplete string) []string {
	files, _ := filepath.Glob(fmt.Sprintf("%s%s", toComplete, "*"))
	return files
}

func isDirectory(s interface{}) error {
	val := s.(string)
	fi, err := os.Stat(val)
	if err != nil {
		return fmt.Errorf("%s: %v", val, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory")
	}
	return nil
}