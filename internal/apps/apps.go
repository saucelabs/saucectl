package apps

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	reFileID = regexp.MustCompile("(storage:(//)?)?(?P<fileID>[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})$")
	reFilePattern = regexp.MustCompile("^(storage:filename=)(?P<filename>[\\S][\\S ]+(\\.ipa|\\.apk))$")
)

func hasValidExtension(file string, exts []string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}

// IsStorageReference checks if a link is an entry of app-storage.
func IsStorageReference(link string) bool {
	return reFileID.MatchString(link) || reFilePattern.MatchString(link)
}

// StandardizeReferenceLink standardize the provided storageID reference to make it work for VMD and RDC.
func StandardizeReferenceLink(storageRef string) string {
	if reFileID.MatchString(storageRef) {
		if !strings.HasPrefix(storageRef, "storage:") {
			return fmt.Sprintf("storage:%s", storageRef)
		}
		if strings.HasPrefix(storageRef, "storage://") {
			return strings.Replace(storageRef, "storage://", "storage:", 1)
		}
	}
	return storageRef
}

// Validate validates that the apps is valid (storageID / File / URL).
func Validate(kind, app string, validExt []string) error {
	if IsStorageReference(app) {
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