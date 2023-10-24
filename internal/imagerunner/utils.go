package imagerunner

import (
	"regexp"
	"strings"
)

func GetCanonicalServiceName(serviceName string) string {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return ""
	}
	// make sure the service name has only lowercase letters, numbers, and underscores
	canonicalName := strings.ToLower(regexp.MustCompile("[^a-zA-Z0-9-]").ReplaceAllString(serviceName, "-"))
	// remove successives dashes
	canonicalName = regexp.MustCompile("-+").ReplaceAllString(canonicalName, "-")
	// avoids container names starting with dash
	canonicalName = regexp.MustCompile("^-").ReplaceAllString(canonicalName, "s-")
	return canonicalName
}
