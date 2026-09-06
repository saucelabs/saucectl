package authoring

import "encoding/json"

// Run is one execution of a test case, grouped under a build and producing one
// job per target. A run has no status field: completion is inferred from its
// jobs (see Done). Observed on 2026-09-05: the start response carries jobs
// with only id, name, sauceJobId and target; isRdc appears ~15 s later and
// success ~19 s in, a few seconds *before* the underlying Sauce job reaches
// "complete". The run resource is therefore the earliest and authoritative
// completion signal.
type Run struct {
	ID     string `json:"id"`
	OrgID  string `json:"orgId,omitempty"`
	TeamID string `json:"teamId,omitempty"`
	UserID string `json:"userId,omitempty"`
	// TestCaseID is the identifier to poll this run with. It is the run's own
	// test case, which is what the detail endpoint enforces (research R-004).
	TestCaseID string `json:"testCaseId"`
	// Build is the build name as stored by the service. The service decorates
	// the requested name (e.g. "nightly" becomes "nightly - 1"), so this is
	// not byte-identical to what was sent.
	Build        string   `json:"build,omitempty"`
	Jobs         []RunJob `json:"jobs"`
	CreationDate string   `json:"creationDate,omitempty"`
	TestURL      string   `json:"testUrl,omitempty"`
}

// Done reports whether every job has reported an outcome. A run with no jobs
// is not done: nothing has been reported for it.
func (r Run) Done() bool {
	if len(r.Jobs) == 0 {
		return false
	}
	for _, j := range r.Jobs {
		if !j.Done() {
			return false
		}
	}
	return true
}

// Passed reports whether the run is done and every job passed.
func (r Run) Passed() bool {
	if !r.Done() {
		return false
	}
	for _, j := range r.Jobs {
		if !j.Passed() {
			return false
		}
	}
	return true
}

// RunJob is the execution of a run against a single target.
type RunJob struct {
	// ID is internal to the authoring service and is not resolvable through
	// the Sauce job APIs.
	ID string `json:"id"`
	// SauceJobID is the real Sauce Labs job identifier: the one to link to,
	// download artifacts for, and look up builds by.
	SauceJobID string `json:"sauceJobId,omitempty"`
	Name       string `json:"name"`
	Target     Target `json:"target"`
	// URL is unreliable — present on roughly half of observed jobs. Derive
	// the link from SauceJobID instead (research R-006).
	URL string `json:"url,omitempty"`
	// IsRDC at the job level is absent until the job is under way; prefer
	// RealDevice, which also consults the target.
	IsRDC bool `json:"isRdc,omitempty"`
	// Success is nil until the service reports an outcome. This pointer is
	// the crux of the design: with a plain bool an in-flight job would be
	// indistinguishable from a failed one (research R-005).
	Success *bool `json:"success,omitempty"`
	// Error carries infrastructure or assertion failure text.
	Error string `json:"error,omitempty"`
}

// Done reports whether the job has a reported outcome, either way.
func (j RunJob) Done() bool {
	return j.Success != nil || j.Error != ""
}

// Passed reports whether the job has reported success.
func (j RunJob) Passed() bool {
	return j.Success != nil && *j.Success
}

// RealDevice reports whether the job runs on a real device, consulting both
// the job-level flag (absent early on) and the target's flag (present from
// the start).
func (j RunJob) RealDevice() bool {
	return j.IsRDC || j.Target.IsRDC
}

// RunOptions is the request to start a run.
type RunOptions struct {
	// BuildName groups the run's jobs under a build. The service caps it at
	// 100 characters and decorates it (see Run.Build).
	BuildName string
	// TunnelName names an active Sauce Connect tunnel. When empty an explicit
	// JSON null is sent, which clears any tunnel name stored on the test
	// case. This is deliberate: some stored cases carry an empty-string tunnel
	// name, and the service rejects a run of those with SC_TUNNEL_NOT_FOUND
	// unless the field is explicitly nulled (research R-008, Open-3). Omitting
	// the field is therefore never correct.
	TunnelName string
	// Targets overrides the test case's stored run targets. When nil the
	// stored targets apply; a run fails with NO_RUN_TARGETS only when neither
	// exists.
	Targets []Target
}

// MarshalJSON emits the request body, always including scTunnelName so that an
// empty TunnelName becomes an explicit null rather than an omission. See the
// TunnelName field for why omitting is never correct.
func (o RunOptions) MarshalJSON() ([]byte, error) {
	type wire struct {
		BuildName  string   `json:"buildName,omitempty"`
		TunnelName *string  `json:"scTunnelName"`
		Targets    []Target `json:"targets,omitempty"`
	}
	w := wire{BuildName: o.BuildName, Targets: o.Targets}
	if o.TunnelName != "" {
		w.TunnelName = &o.TunnelName
	}
	return json.Marshal(w)
}

// ListRunsOptions filters a run listing. The test case is a method argument,
// not an option, because it is mandatory.
type ListRunsOptions struct {
	ListOptions
	StartDate string
	EndDate   string
	UserID    string
	TeamID    string
}
