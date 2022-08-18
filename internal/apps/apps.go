package apps

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/saucelabs/saucectl/internal/msg"
)

var (
	reFileID            = regexp.MustCompile(`(storage:(//)?)?(?P<fileID>[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})$`)
	reFilePattern       = regexp.MustCompile(`^(storage:filename=)(?P<filename>[\S][\S ]+(\.ipa|\.apk))$`)
	reHttpSchemePattern = regexp.MustCompile(`(?i)^https?`)
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
	return reHttpSchemePattern.MatchString(parsedURL.Scheme) && parsedURL.Host != ""
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

	if IsRemote(app) {
		return nil
	}

	if !hasValidExtension(app, validExt) {
		return fmt.Errorf("invalid %s file: %s, make sure extension is one of the following: %s", kind, app, strings.Join(validExt, ", "))
	}

	if _, err := os.Stat(app); err == nil {
		return nil
	}
	return fmt.Errorf(msg.FileNotFound, app)
}

// Download downloads a file from remoteUrl to a temp directory and returns the path to the downloaded file
func Download(remoteUrl string) (string, error) {
	resp, err := http.Get(remoteUrl)

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unable to download app from %s: %s", remoteUrl, resp.Status)
	}

	dir, err := os.MkdirTemp("", "tmp-app")
	if err != nil {
		return "", err
	}

	tmpFilePath := path.Join(dir, path.Base(remoteUrl))

	f, err := os.Create(tmpFilePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", nil
	}

	return tmpFilePath, nil
}
