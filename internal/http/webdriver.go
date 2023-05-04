package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/slice"
	"github.com/saucelabs/saucectl/internal/version"
)

// Webdriver service
type Webdriver struct {
	HTTPClient  *http.Client
	URL         string
	Credentials iam.Credentials
}

// SessionRequest represents the webdriver session request.
type SessionRequest struct {
	Capabilities        Capabilities `json:"capabilities,omitempty"`
	DesiredCapabilities MatchingCaps `json:"desiredCapabilities,omitempty"`
}

// Capabilities represents the webdriver capabilities.
// https://www.w3.org/TR/webdriver/
type Capabilities struct {
	AlwaysMatch MatchingCaps `json:"alwaysMatch,omitempty"`
}

// MatchingCaps are specific attributes that together form the capabilities that are used to match a session.
type MatchingCaps struct {
	App               string    `json:"app,omitempty"`
	OtherApps         []string  `json:"otherApps,omitempty"`
	BrowserName       string    `json:"browserName,omitempty"`
	BrowserVersion    string    `json:"browserVersion,omitempty"`
	PlatformName      string    `json:"platformName,omitempty"`
	SauceOptions      SauceOpts `json:"sauce:options,omitempty"`
	PlatformVersion   string    `json:"platformVersion,omitempty"`
	DeviceName        string    `json:"deviceName,omitempty"`
	DeviceOrientation string    `json:"deviceOrientation,omitempty"`
}

// SauceOpts represents the Sauce Labs specific capabilities.
type SauceOpts struct {
	DevX             bool     `json:"devX,omitempty"`
	TestName         string   `json:"name,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	BuildName        string   `json:"build,omitempty"`
	Batch            Batch    `json:"_batch,omitempty"`
	IdleTimeout      int      `json:"idleTimeout,omitempty"`
	MaxDuration      int      `json:"maxDuration,omitempty"`
	TunnelIdentifier string   `json:"tunnelIdentifier,omitempty"`
	TunnelParent     string   `json:"parentTunnel,omitempty"` // note that 'parentTunnel` is backwards, because that's the way sauce likes it
	ScreenResolution string   `json:"screen_resolution,omitempty"`
	SauceCloudNode   string   `json:"_sauceCloudNode,omitempty"`
	UserAgent        string   `json:"user_agent,omitempty"`
	TimeZone         string   `json:"timeZone,omitempty"`
	Visibility       string   `json:"public,omitempty"`
}

// Batch represents capabilities for batch frameworks.
type Batch struct {
	Framework        string              `json:"framework,omitempty"`
	FrameworkVersion string              `json:"frameworkVersion,omitempty"`
	RunnerVersion    string              `json:"runnerVersion,omitempty"`
	TestFile         string              `json:"testFile,omitempty"`
	Args             []map[string]string `json:"args"`
	VideoFPS         int                 `json:"video_fps"`
}

// sessionStartResponse represents the response body for starting a session.
type sessionStartResponse struct {
	Status    int    `json:"status,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Value     struct {
		Message string `json:"message,omitempty"`
	} `json:"value,omitempty"`
}

func NewWebdriver(url string, creds iam.Credentials, timeout time.Duration) Webdriver {
	return Webdriver{
		HTTPClient: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Sauce can queue up Job start requests for up to 10 minutes and sends redirects in the meantime to
				// keep the connection alive. A redirect is sent every 45 seconds.
				// 10m / 45s requires a minimum of 14 redirects.
				if len(via) >= 20 {
					return errors.New("stopped after 20 redirects")
				}

				return nil
			},
		},
		URL:         url,
		Credentials: creds,
	}
}

// StartJob creates a new job in Sauce Labs.
func (c *Webdriver) StartJob(ctx context.Context, opts job.StartOptions) (jobID string, isRDC bool, err error) {
	url := fmt.Sprintf("%s/wd/hub/session", c.URL)

	caps := Capabilities{AlwaysMatch: MatchingCaps{
		App:             opts.App,
		OtherApps:       opts.OtherApps,
		BrowserName:     c.normalizeBrowser(opts.Framework, opts.BrowserName),
		BrowserVersion:  opts.BrowserVersion,
		PlatformName:    opts.PlatformName,
		PlatformVersion: opts.PlatformVersion,
		SauceOptions: SauceOpts{
			UserAgent:        "saucectl/" + version.Version,
			DevX:             true,
			TunnelIdentifier: opts.Tunnel.ID,
			TunnelParent:     opts.Tunnel.Parent,
			ScreenResolution: opts.ScreenResolution,
			SauceCloudNode:   opts.Experiments["_sauceCloudNode"],
			TestName:         opts.Name,
			BuildName:        opts.Build,
			Tags:             opts.Tags,
			Batch: Batch{
				Framework:        opts.Framework,
				FrameworkVersion: opts.FrameworkVersion,
				RunnerVersion:    opts.RunnerVersion,
				TestFile:         opts.Suite,
				Args:             c.formatEspressoArgs(opts.TestOptions),
				VideoFPS:         13, // 13 is the sweet spot to minimize frame drops
			},
			IdleTimeout: 9999,
			MaxDuration: 10800,
			TimeZone:    opts.TimeZone,
			Visibility:  opts.Visibility,
		},
		DeviceName:        opts.DeviceName,
		DeviceOrientation: opts.DeviceOrientation,
	}}

	// Emulator/Simulator requests are allergic to W3C capabilities. Requests get routed to RDC. However, using legacy
	// format alone is insufficient, we need both.
	session := SessionRequest{
		Capabilities:        caps,
		DesiredCapabilities: caps.AlwaysMatch,
	}

	var b bytes.Buffer
	err = json.NewEncoder(&b).Encode(session)
	if err != nil {
		return
	}

	req, err := NewRequestWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return
	}
	req.SetBasicAuth(c.Credentials.Username, c.Credentials.AccessKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var sessionStart sessionStartResponse
	if err = json.Unmarshal(body, &sessionStart); err != nil {
		return "", false, fmt.Errorf("job start failed (%d): %s", resp.StatusCode, body)
	}

	if sessionStart.SessionID == "" {
		err = fmt.Errorf("job start failed (%d): %s", resp.StatusCode, sessionStart.Value.Message)
		return "", false, err
	}

	return sessionStart.SessionID, false, nil
}

// formatEspressoArgs adapts option shape to match chef expectations
func (c *Webdriver) formatEspressoArgs(options map[string]interface{}) []map[string]string {
	var mappedOptions []map[string]string
	for k, v := range options {
		if v == nil {
			continue
		}

		value := fmt.Sprintf("%v", v)

		// class/notClass need special treatment, because we accept these as slices, but the backend wants
		// a comma separated string.
		if k == "class" || k == "notClass" {
			value = slice.Join(v, ",")
		}

		if value == "" {
			continue
		}
		mappedOptions = append(mappedOptions, map[string]string{
			"name":  k,
			"value": value,
		})
	}
	return mappedOptions
}

// normalizeBrowser converts the user specified browsers into something Sauce Labs can understand better.
func (c *Webdriver) normalizeBrowser(framework, browser string) string {
	switch framework {
	case "cypress":
		switch browser {
		case "chrome":
			return "googlechrome"
		case "webkit":
			return "cypress-webkit"
		}
	case "testcafe":
		switch browser {
		case "chrome":
			return "googlechrome"
		}
	case "playwright":
		switch browser {
		case "chrome":
			return "googlechrome"
		case "chromium":
			return "playwright-chromium"
		case "firefox":
			return "playwright-firefox"
		case "webkit":
			return "playwright-webkit"
		}
	}
	return browser
}
