package usage

import (
	"io"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/spf13/pflag"
)

// Properties is a scoped data transfer object for usage reporting and contains usage event related data.
type Properties map[string]interface{}

// SetArtifacts reports artifact usage.
func (p Properties) SetArtifacts(art config.Artifacts) Properties {
	p["artifact_download_match"] = art.Download.Match
	p["artifact_download_when"] = art.Download.When
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

// SetNPM reports the npm usage.
func (p Properties) SetNPM(npm config.Npm) Properties {
	p["npm_registry"] = npm.Registry

	var pkgs []string
	for k := range npm.Packages {
		pkgs = append(pkgs, k)
	}
	p["npm_packages"] = pkgs
	p["npm_dependencies"] = npm.Dependencies

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
	p["tunnel"] = c.Tunnel.Name
	p["tunnel_owner"] = c.Tunnel.Owner
	p["retries"] = c.Retries

	return p
}

// SetSlack reports Slack related settings.
func (p Properties) SetSlack(slack config.Slack) Properties {
	p["slack_channels_count"] = len(slack.Channels)
	p["slack_when"] = slack.Send
	return p
}

func (p Properties) SetSharding(sharded bool) Properties {
	p["sharded"] = sharded
	return p
}

// SetLaunchOrder reports launch order of jobs
func (p Properties) SetLaunchOrder(launchOrder config.LaunchOrder) Properties {
	p["launch_order"] = string(launchOrder)
	return p
}

// SetSmartRetry reports if the suites set as smartRetry
func (p Properties) SetSmartRetry(isSmartRetried bool) Properties {
	p["smart_retry_failed_only"] = isSmartRetried
	return p
}

func (p Properties) SetReporters(reporters config.Reporters) Properties {
	p["reporters_spotlight_enabled"] = reporters.Spotlight.Enabled
	p["reporters_junit_enabled"] = reporters.JUnit.Enabled
	p["reporters_json_enabled"] = reporters.JSON.Enabled
	return p
}

// Tracker is an interface for providing usage tracking.
type Tracker interface {
	io.Closer
	Collect(subject string, props Properties)
}
