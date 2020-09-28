package gitlab

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
	CIPipelineID string
	CIJobStage   string
}

// FromEnv creates a new Pipeline from the environment.
func FromEnv() ci.Provider {
	return Pipeline{
		CIPipelineID: os.Getenv("CI_PIPELINE_ID"),
		CIJobStage:   os.Getenv("CI_JOB_STAGE"),
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
	_, ok := os.LookupEnv("GITLAB_CI")
	return ok
}

// Enable enables this provider integration.
func Enable() {
	ci.RegisterProvider("GitLab", Available, FromEnv)
}
