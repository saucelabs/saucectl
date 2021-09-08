package usage

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/spf13/pflag"
	"io"
)

// Properties is a scoped data transfer object for usage reporting and contains usage event related data.
type Properties map[string]interface{}

// SetArtifacts reports artifact usage.
func (p Properties) SetArtifacts(art config.Artifacts) Properties {
	p["artifact_download_match"] = art.Download.Match
	p["artifact_download_when"] = art.Download.When
	return p
}

// SetDocker reports the docker container setup.
func (p Properties) SetDocker(d config.Docker) Properties {
	p["docker_img"] = d.Image
	p["docker_transfer"] = d.FileTransfer
	return p
}

// SetFramework reports the framework.
func (p Properties) SetFramework(f string) Properties {
	p["framework"] = f
	return p
}

// SetFVersion reports the framework version.
func (p Properties) SetFVersion(f string) Properties {
	p["framework_version"] = f
	return p
}

// SetFlags reports CLI flags.
func (p Properties) SetFlags(flags *pflag.FlagSet) Properties {
	var ff []string

	flags.Visit(func(flag *pflag.Flag) {
		ff = append(ff, flag.Name)
	})

	p["flags"] = ff

	return p
}

// SetJobs reports job (aka test results).
func (p Properties) SetJobs(jobs []report.TestResult) Properties {
	p["jobs"] = jobs

	return p
}

// SetNPM reports the npm usage.
func (p Properties) SetNPM(npm config.Npm) Properties {
	p["npm_registry"] = npm.Registry

	var pkgs []string
	for k := range npm.Packages {
		pkgs = append(pkgs, k)
	}
	p["npm_packages"] = pkgs

	return p
}

// SetNumSuites reports the number of configured suites.
func (p Properties) SetNumSuites(n int) Properties {
	p["num_suites"] = n
	return p
}

// SetSauceConfig reports key fields of the sauce config.
func (p Properties) SetSauceConfig(c config.SauceConfig) Properties {
	p["concurrency"] = c.Concurrency
	p["region"] = c.Region
	p["tunnel"] = c.Tunnel.ID
	p["tunnel_owner"] = c.Tunnel.Parent
	p["retries"] = c.Retries

	return p
}

// Tracker is an interface for providing usage tracking.
type Tracker interface {
	io.Closer
	Collect(subject string, props Properties)
}
