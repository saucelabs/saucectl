package sentry

import (
	"bytes"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog/log"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Scope represents the scope (aka context) in which the error/event occurred.
type Scope struct {
	// Sauce Labs username.
	Username string
	// ConfigFile represents the path to the config.yml file.
	ConfigFile string
}

// CaptureError captures the given error in sentry.
func CaptureError(err error, scope Scope) {
	sentry.ConfigureScope(func(sentryScope *sentry.Scope) {
		sentryScope.SetUser(sentry.User{ID: scope.Username})
		sentryScope.AddEventProcessor(func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			attach(http.Client{Timeout: 10 * time.Second}, string(event.EventID), scope.ConfigFile)
			return event
		})
	})
	sentry.CaptureException(err)
	sentry.Flush(3 * time.Second)
}

func attach(client http.Client, eventID, filename string) {
	if filename == "" {
		return
	}

	r, err := os.Open(filename)
	if err != nil {
		log.Debug().Err(err).Msgf("Failed to open %s", filename)
		return
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreatePart(newMIMEHeader("attachment", filepath.Base(filename), "text/plain"))
	if err != nil {
		log.Debug().Err(err).Msgf("Failed to create form part %s", filename)
		return
	}
	_, _ = io.Copy(fw, r)
	_ = w.Close()

	sentryURL, err := attachmentURLFromDSN(sentry.CurrentHub().Client().Options().Dsn, eventID)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to assemble the sentry attachment URL")
		return
	}

	req, err := http.NewRequest("POST", sentryURL, &b)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to create sentry HTTP request")
		return
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Submit the request
	res, err := client.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to send sentry attachment")
		return
	}

	// Check the response
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		log.Debug().Int("status", res.StatusCode).
			Msgf("Failed to send sentry attachment. Sentry responded with: %s", b)
	}
}

func newMIMEHeader(fieldname, filename, contentType string) textproto.MIMEHeader {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(fieldname), escapeQuotes(filename)))
	h.Set("Content-Type", contentType)
	return h
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func attachmentURLFromDSN(dsn string, eventID string) (string, error) {
	parsedURL, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("invalid url: %v", err)
	}

	// PublicKey
	publicKey := parsedURL.User.Username()
	if publicKey == "" {
		return "", fmt.Errorf("empty username")
	}

	// ProjectID
	if len(parsedURL.Path) == 0 || parsedURL.Path == "/" {
		return "", fmt.Errorf("empty project id")
	}
	pathSegments := strings.Split(parsedURL.Path[1:], "/")
	projectID, err := strconv.Atoi(pathSegments[len(pathSegments)-1])
	if err != nil {
		return "", fmt.Errorf("invalid project id")
	}

	// Path
	var path string
	if len(pathSegments) > 1 {
		path = "/" + strings.Join(pathSegments[0:len(pathSegments)-1], "/")
	}

	return fmt.Sprintf("%s://%s%s/api/%d/events/%s/attachments/?sentry_key=%s",
		parsedURL.Scheme, parsedURL.Host, path, projectID, eventID, publicKey), nil
}
