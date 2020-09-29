package ci

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

var providers = make(map[string]holder)

// IsAvailable detects whether this code is executed inside a CI environment
func IsAvailable() bool {
	// Most CI providers have this.
	isCi := os.Getenv("CI") != "" || os.Getenv("BUILD_NUMBER") != ""
	skip := os.Getenv("SKIP_CI") != ""
	return isCi && !skip
}

// holder holds CI provider initialization method references. These are used to lazily initialize the specific CI
// provider.
type holder struct {
	available func() bool
	create    func() Provider
}

// Provider represents the CI provider.
type Provider interface {
	BuildID() string
	// SetBuildID overrides the CI provided build ID.
	SetBuildID(id string)
}

// RegisterProvider registers a "create" function that returns a new instance of the given Provider function.
// The "available" function should determine whether this Provider is actually available in the current environment.
func RegisterProvider(name string, available func() bool, create func() Provider) {
	providers[name] = holder{
		available: available,
		create:    create,
	}
}

// Detect detects which CI environment this code is executed in.
// It can only find the providers that were previous registered by calling RegisterProvider().
// Returns NoProvider if no supported CI provider could be detected.
func Detect() Provider {
	for _, p := range providers {
		if p.available() {
			return p.create()
		}
	}

	return NoProvider
}

// NoProvider represents a NO-OP provider for cases where no supported CI provider was detected.
var NoProvider = &fakePipeline{CreatedAt: time.Now()}

// fakePipeline represents a NO-OP provider for cases where no supported CI provider was detected.
type fakePipeline struct {
	CreatedAt       time.Time
	overrideBuildID string
}

// SetBuildID overrides the return value for BuildID().
func (p *fakePipeline) SetBuildID(id string) {
	p.overrideBuildID = id
}

// BuildID returns a build ID.
func (p fakePipeline) BuildID() string {
	if p.overrideBuildID != "" {
		return p.overrideBuildID
	}

	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%+v", p))
	return hex.EncodeToString(h.Sum(nil))
}
