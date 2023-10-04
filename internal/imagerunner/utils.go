package imagerunner

import (
	"regexp"
	"strings"
)

func GetCanonicalServiceName(serviceName string) string {
	if serviceName == "" {
		return ""
	}
	// make sure the service name has only lowercase letters, numbers, and underscores
	canonicalName := strings.ToLower(regexp.MustCompile("[^a-z0-9-]").ReplaceAllString(serviceName, "-"))
	// remove successives dashes
	canonicalName = regexp.MustCompile("-+").ReplaceAllString(canonicalName, "-")
	return canonicalName
}
