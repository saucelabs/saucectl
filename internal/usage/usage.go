package usage

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/spf13/pflag"
	"gopkg.in/segmentio/analytics-go.v3"
)

// Properties is a scoped data transfer object for usage reporting and contains usage event related data.
type Properties = analytics.Properties

// Option is a function that configures a Properties instance.
type Option func(Properties)

func Artifacts(art config.Artifacts) Option {
	return func(p Properties) {
		p["artifact_download_match"] = art.Download.Match
		p["artifact_download_when"] = art.Download.When
	}
}

func Framework(name, version string) Option {
	return func(p Properties) {
		p["framework"] = name
		if version != "" {
			p["framework_version"] = version
		}
	}
}

func Flags(flags *pflag.FlagSet) Option {
	return func(p Properties) {
		var ff []string
		flags.Visit(func(flag *pflag.Flag) {
			ff = append(ff, flag.Name)
		})
		p["flags"] = ff
	}
}

func NPM(npm config.Npm) Option {
	return func(p Properties) {
		p["npm_registry"] = npm.Registry

		var pkgs []string
		for k := range npm.Packages {
			pkgs = append(pkgs, k)
		}
		p["npm_packages"] = pkgs
		p["npm_dependencies"] = npm.Dependencies
		p["npm_use_package_lock"] = npm.UsePackageLock
	}
}

func NumSuites(n int) Option {
	return func(p Properties) {
		p["num_suites"] = n
	}
}

func SauceConfig(c config.SauceConfig) Option {
	return func(p Properties) {
		p["concurrency"] = c.Concurrency
		p["region"] = c.Region
		p["tunnel"] = c.Tunnel.Name
		p["tunnel_owner"] = c.Tunnel.Owner
		p["retries"] = c.Retries
		p["launch_order"] = string(c.LaunchOrder)
	}
}

func Slack(slack config.Slack) Option {
	return func(p Properties) {
		p["slack_channels_count"] = len(slack.Channels)
		p["slack_when"] = slack.Send
	}
}

func Sharding(shardTypes []string, shardOpts map[string]bool) Option {
	return func(p Properties) {
		p["sharded"] = len(shardTypes) > 0
		p["shard_types"] = shardTypes
		for k, v := range shardOpts {
			p[k] = v
		}
	}
}

func SmartRetry(isSmartRetried bool) Option {
	return func(p Properties) {
		p["smart_retry_failed_only"] = isSmartRetried
	}
}

func Reporters(reporters config.Reporters) Option {
	return func(p Properties) {
		p["reporters_spotlight_enabled"] = reporters.Spotlight.Enabled
		p["reporters_junit_enabled"] = reporters.JUnit.Enabled
		p["reporters_json_enabled"] = reporters.JSON.Enabled
	}
}

func Node(version string) Option {
	return func(p Properties) {
		p["node_version"] = version
	}
}
