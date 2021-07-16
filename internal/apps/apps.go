package apps

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

func hasValidExtension(file string, exts []string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}

// Validate validates that the apps is valid (storageID / File / URL).
func Validate(kind, app string, validExt []string, URLAllowed bool) error {
	re := regexp.MustCompile("^(storage:(//)?)?[0-9a-f]{8}-([0-9a-f]{4}-){3}[0-9a-f]{12}$")
	if re.MatchString(app) {
		return nil
	}

	if _, err := url.ParseRequestURI(app); URLAllowed && err == nil {
		return nil
	}

	if !hasValidExtension(app, validExt) {
		return fmt.Errorf("invalid %s file: %s, make sure extension is one of the following: %s", kind, app, strings.Join(validExt, ", "))
	}

	if _, err := os.Stat(app); err == nil {
		return nil
	}
	return fmt.Errorf("%s: file not found", app)
}