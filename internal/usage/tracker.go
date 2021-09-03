package usage

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/report"
	"io"
)

type Properties map[string]interface{}

func (p Properties) Set(key string, val interface{}) Properties {
	p[key] = val
	return p
}

func (p Properties) SetArtifacts(art config.Artifacts) Properties {
	p["artifact_download_match"] = art.Download.Match
	p["artifact_download_when"] = art.Download.When
	return p
}

func (p Properties) SetDocker(d config.Docker) Properties {
	p["docker_img"] = d.Image
	p["docker_transfer"] = d.FileTransfer
	return p
}

// SetFramework sets the test framework.
func (p Properties) SetFramework(f string) Properties {
	p["framework"] = f
	return p
}

// SetFVersion sets the test framework version.
func (p Properties) SetFVersion(f string) Properties {
	p["framework_version"] = f
	return p
}

func (p Properties) SetFlags(flags map[string]interface{}) Properties {
	p["flags"] = flags
	return p
}

func (p Properties) SetJobs(jobs []report.TestResult) Properties {
	p["jobs"] = jobs

	return p
}

func (p Properties) SetNPM(npm config.Npm) Properties {
	p["npm_registry"] = npm.Registry
	p["npm_packages"] = npm.Packages
	return p
}

func (p Properties) SetNumSuites(n int) Properties {
	p["num_suites"] = n
	return p
}

func (p Properties) SetSauceConfig(c config.SauceConfig) Properties {
	p["concurrency"] = c.Concurrency
	p["region"] = c.Region
	p["tunnel"] = c.Tunnel.ID
	p["tunnel_owner"] = c.Tunnel.Parent
	p["retries"] = c.Retries

	return p
}

type Tracker interface {
	io.Closer
	Collect(subject string, props Properties)
}
