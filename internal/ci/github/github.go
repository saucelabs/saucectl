package github

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/saucelabs/saucectl/internal/ci"
	"io"
	"os"
)

// Pipeline represents the current pipeline information.
type Pipeline struct {
	GithubWorkflow string
	GithubRunID    string
	InvocationID   string
}

// FromEnv creates a new Pipeline from the environment.
func FromEnv() ci.Provider {
	return Pipeline{
		GithubWorkflow: os.Getenv("GITHUB_WORKFLOW"),
		GithubRunID:    os.Getenv("GITHUB_RUN_ID"),
		InvocationID:   os.Getenv("INVOCATION_ID"),
	}
}

// BuildID returns a build ID.
func (p Pipeline) BuildID() string {
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%+v", p))
	return hex.EncodeToString(h.Sum(nil))
}

// Available returns true if the code is executed in a github CI environment.
func Available() bool {
	_, ok := os.LookupEnv("GITHUB_RUN_ID")
	return ok
}

// Enable enables this provider integration.
func Enable() {
	ci.RegisterProvider("GitHub", Available, FromEnv)
}