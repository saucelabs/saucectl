package authoring

import (
	"encoding/json"
	"strings"
)

// TestSchedule is a recurring trigger that runs one or more suites on a cron
// pattern in a stated timezone.
type TestSchedule struct {
	ID                   string           `json:"id"`
	OrgID                string           `json:"orgId,omitempty"`
	TeamID               string           `json:"teamId,omitempty"`
	CreatorUserID        string           `json:"creatorUserId,omitempty"`
	CreatorUserName      string           `json:"creatorUserName,omitempty"`
	CreationDate         string           `json:"creationDate,omitempty"`
	LastModifierUserID   string           `json:"lastModifierUserId,omitempty"`
	LastModifierUserName string           `json:"lastModifierUserName,omitempty"`
	LastUpdateDate       string           `json:"lastUpdateDate,omitempty"`
	Name                 string           `json:"name"`
	Settings             ScheduleSettings `json:"settings"`
	State                ScheduleState    `json:"state"`
	TestSuiteIDs         []string         `json:"testSuiteIds"`
}

// ScheduleSettings is when and how a schedule runs. Cron, Timezone and
// RunningUserID are required by the service; the rest are optional bounds.
type ScheduleSettings struct {
	// Cron is a six-field cron expression with seconds first, as observed on
	// live schedules ("0 0 10 * * *" = 10:00:00 daily). The data model's
	// earlier "five-field" note was wrong.
	Cron string `json:"cron"`
	// Timezone is an IANA zone name such as "Europe/Berlin".
	Timezone string `json:"timezone"`
	// RunningUserID is the user the scheduled runs execute as.
	RunningUserID string `json:"runningUserId"`
	StartDate     string `json:"startDate,omitempty"`
	EndDate       string `json:"endDate,omitempty"`
	// MaxRuns is a pointer so that "no limit" (nil) is distinct from a limit
	// of zero.
	MaxRuns    *int   `json:"maxRuns,omitempty"`
	TunnelName string `json:"scTunnelName,omitempty"`
	BuildName  string `json:"buildName,omitempty"`
}

// ScheduleState is the schedule's observed runtime state.
type ScheduleState struct {
	StateName     ScheduleStateName `json:"stateName"`
	LastRunError  string            `json:"lastRunError,omitempty"`
	LastRunDate   string            `json:"lastRunDate,omitempty"`
	NextRunDate   string            `json:"nextRunDate,omitempty"`
	RemainingRuns *int              `json:"remainingRuns,omitempty"`
}

// ScheduleStateName is a schedule's lifecycle state.
type ScheduleStateName string

// The schedule states. Only Enabled and Disabled can be set by a user;
// Running and Errored are observed. Reaching MaxRuns or passing EndDate
// disables a schedule.
const (
	ScheduleEnabled  ScheduleStateName = "ENABLED"
	ScheduleDisabled ScheduleStateName = "DISABLED"
	ScheduleErrored  ScheduleStateName = "ERRORED"
	ScheduleRunning  ScheduleStateName = "RUNNING"
)

// SettableScheduleStates are the states a user may request.
var SettableScheduleStates = []ScheduleStateName{ScheduleEnabled, ScheduleDisabled}

// ParseScheduleState resolves a user-supplied state case-insensitively and
// reports whether it is one a user may set.
func ParseScheduleState(s string) (ScheduleStateName, bool) {
	name := ScheduleStateName(strings.ToUpper(strings.TrimSpace(s)))
	for _, st := range SettableScheduleStates {
		if st == name {
			return st, true
		}
	}
	return name, false
}

// ListSchedulesOptions filters a schedule listing.
type ListSchedulesOptions struct {
	ListOptions
	IDs          []string
	Search       string
	StartDate    string
	EndDate      string
	UserID       string
	TeamID       string
	TestSuiteIDs []string
}

// CreateScheduleOptions is the request to create a schedule.
type CreateScheduleOptions struct {
	Name         string            `json:"name"`
	Settings     ScheduleSettings  `json:"settings"`
	TestSuiteIDs []string          `json:"testSuiteIds"`
	StateName    ScheduleStateName `json:"stateName"`
}

// UpdateScheduleOptions is the request to update a schedule. The service
// replaces the schedule and rejects a partial body with INVALID_BODY naming
// the missing fields (observed 2026-09-06, research Open-4), so callers
// perform read-modify-write and always send Name, a fully populated Settings,
// the full TestSuiteIDs and StateName. AddTestSuiteIDs / RemoveTestSuiteIDs
// mirror the documented request shape but are not relied upon.
type UpdateScheduleOptions struct {
	Name               string                 `json:"name,omitempty"`
	Settings           *ScheduleSettingsPatch `json:"settings,omitempty"`
	TestSuiteIDs       []string               `json:"testSuiteIds,omitempty"`
	AddTestSuiteIDs    []string               `json:"addTestSuiteIds,omitempty"`
	RemoveTestSuiteIDs []string               `json:"removeTestSuiteIds,omitempty"`
	StateName          ScheduleStateName      `json:"stateName,omitempty"`
}

// ScheduleSettingsPatch is the settings object of an update. Every optional
// field is tri-state, because that is how the service behaves (observed
// 2026-09-06): an omitted field keeps its stored value, an explicit null
// clears it — for maxRuns, startDate and endDate too, although the
// specification marks them non-nullable — and an empty-string date is
// rejected. So nil omits, a pointer to "" (or ClearMaxRuns) sends null, and
// any other value is sent as is. Cron, Timezone and RunningUserID are
// required by the service and always sent.
type ScheduleSettingsPatch struct {
	Cron          string  `json:"cron"`
	Timezone      string  `json:"timezone"`
	RunningUserID string  `json:"runningUserId"`
	StartDate     *string `json:"-"`
	EndDate       *string `json:"-"`
	MaxRuns       *int    `json:"-"`
	// ClearMaxRuns sends maxRuns as null. A separate flag because zero is a
	// real value the service stores, not "unlimited".
	ClearMaxRuns bool    `json:"-"`
	TunnelName   *string `json:"-"`
	BuildName    *string `json:"-"`
}

// PatchFromSettings converts stored settings into a patch that re-sends every
// populated field, the starting point for read-modify-write.
func PatchFromSettings(s ScheduleSettings) ScheduleSettingsPatch {
	p := ScheduleSettingsPatch{
		Cron:          s.Cron,
		Timezone:      s.Timezone,
		RunningUserID: s.RunningUserID,
		MaxRuns:       s.MaxRuns,
	}
	p.StartDate = optionalString(s.StartDate)
	p.EndDate = optionalString(s.EndDate)
	p.TunnelName = optionalString(s.TunnelName)
	p.BuildName = optionalString(s.BuildName)
	return p
}

// optionalString returns nil for "" and a pointer to the value otherwise, so
// an unset stored field is omitted rather than nulled.
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// MarshalJSON implements the tri-state encoding of the optional fields.
func (p ScheduleSettingsPatch) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"cron":          p.Cron,
		"timezone":      p.Timezone,
		"runningUserId": p.RunningUserID,
	}
	if p.StartDate != nil {
		m["startDate"] = nullable(*p.StartDate)
	}
	if p.EndDate != nil {
		m["endDate"] = nullable(*p.EndDate)
	}
	switch {
	case p.ClearMaxRuns:
		m["maxRuns"] = nil
	case p.MaxRuns != nil:
		m["maxRuns"] = *p.MaxRuns
	}
	if p.TunnelName != nil {
		m["scTunnelName"] = nullable(*p.TunnelName)
	}
	if p.BuildName != nil {
		m["buildName"] = nullable(*p.BuildName)
	}
	return json.Marshal(m)
}

// nullable maps the empty string to JSON null and any other string to itself.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
