package apps

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	reFileID            = regexp.MustCompile(`(storage:(//)?)?(?P<fileID>[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})$`)
	reFilePattern       = regexp.MustCompile(`^(storage:filename=)(?P<filename>[\S][\S ]+(\.ipa|\.apk|\.zip))$`)
	reHTTPSchemePattern = regexp.MustCompile(`(?i)^https?`)
)

func hasValidExtension(file string, exts []string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}

// IsRemote (naively) checks if the given string is a remote url
func IsRemote(name string) bool {
	parsedURL, err := url.Parse(name)
	if err != nil {
		return false
	}
	return reHTTPSchemePattern.MatchString(parsedURL.Scheme) && parsedURL.Host != ""
}

// IsStorageReference checks if a link is an entry of app-storage.
func IsStorageReference(link string) bool {
	return reFileID.MatchString(link) || reFilePattern.MatchString(link)
}

// NormalizeStorageReference normalizes ref to work across VMD and RDC.
func NormalizeStorageReference(ref string) string {
	if reFileID.MatchString(ref) {
		if !strings.HasPrefix(ref, "storage:") {
			return fmt.Sprintf("storage:%s", ref)
		}
		if strings.HasPrefix(ref, "storage://") {
			return strings.Replace(ref, "storage://", "storage:", 1)
		}
	}
	return ref
}

// Validate validates that app is valid (storageID / File / URL).
func Validate(kind, app string, validExt []string) error {
	if IsStorageReference(app) {
		return nil
	}

	if IsRemote(app) {
		return nil
	}

	if !hasValidExtension(app, validExt) {
		return fmt.Errorf("invalid %s file: %s, make sure extension is one of the following: %s", kind, app, strings.Join(validExt, ", "))
	}

	return nil
}
