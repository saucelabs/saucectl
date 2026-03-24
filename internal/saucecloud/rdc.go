// rdc.go holds shared utilities for Real Device Cloud (RDC) runners (Espresso, XCUITest, XCTest).
// RDC and Virtual Machine Device (VMD) runners do not have feature parity; certain capabilities
// such as network throttling are only available on real devices. This file consolidates
// RDC-specific helpers that are shared across multiple runners but do not belong in cloud.go,
// which is reserved for infrastructure shared by all runners (RDC and VMD alike).
package saucecloud

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
)

// configToJobNetworkConditions converts config-layer network conditions (YAML camelCase)
// to job-layer network conditions (JSON snake_case) for the RDC API request payload.
// See: https://docs.saucelabs.com/mobile-apps/features/network-throttling/
func configToJobNetworkConditions(nc *config.NetworkConditions) *job.NetworkConditions {
	if nc == nil {
		return nil
	}
	return &job.NetworkConditions{
		DownloadSpeed: nc.DownloadSpeed,
		UploadSpeed:   nc.UploadSpeed,
		Latency:       nc.Latency,
		Loss:          nc.Loss,
	}
}
