package authoring

import (
	"encoding/json"
	"testing"
)

func TestScheduleSettingsPatch_MarshalJSON(t *testing.T) {
	empty := ""
	tunnel := "my-tunnel"
	three := 3

	tests := []struct {
		name  string
		patch ScheduleSettingsPatch
		want  string
	}{
		{
			name:  "nil pointers omit every optional field; required ones always travel",
			patch: ScheduleSettingsPatch{Cron: "0 * * * *", Timezone: "Europe/Berlin", RunningUserID: "u"},
			want:  `{"cron":"0 * * * *","runningUserId":"u","timezone":"Europe/Berlin"}`,
		},
		{
			name:  "pointer to empty string sends null to clear",
			patch: ScheduleSettingsPatch{Cron: "c", Timezone: "tz", RunningUserID: "u", TunnelName: &empty, BuildName: &empty, StartDate: &empty, EndDate: &empty},
			want:  `{"buildName":null,"cron":"c","endDate":null,"runningUserId":"u","scTunnelName":null,"startDate":null,"timezone":"tz"}`,
		},
		{
			name:  "ClearMaxRuns sends null, a value sends the number, zero is a real value",
			patch: ScheduleSettingsPatch{Cron: "c", Timezone: "tz", RunningUserID: "u", ClearMaxRuns: true},
			want:  `{"cron":"c","maxRuns":null,"runningUserId":"u","timezone":"tz"}`,
		},
		{
			name:  "values are sent as is",
			patch: ScheduleSettingsPatch{Cron: "c", Timezone: "tz", RunningUserID: "u", TunnelName: &tunnel, MaxRuns: &three},
			want:  `{"cron":"c","maxRuns":3,"runningUserId":"u","scTunnelName":"my-tunnel","timezone":"tz"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.patch)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tt.want {
				t.Errorf("got  %s\nwant %s", b, tt.want)
			}
		})
	}
}

func TestPatchFromSettings(t *testing.T) {
	five := 5
	p := PatchFromSettings(ScheduleSettings{Cron: "c", Timezone: "tz", RunningUserID: "u", MaxRuns: &five, TunnelName: "t"})
	if p.TunnelName == nil || *p.TunnelName != "t" {
		t.Errorf("tunnel not carried over: %v", p.TunnelName)
	}
	if p.BuildName != nil || p.StartDate != nil || p.EndDate != nil {
		t.Errorf("empty stored optional fields must become nil (omitted): %+v", p)
	}
	if p.MaxRuns == nil || *p.MaxRuns != 5 {
		t.Error("maxRuns not carried over")
	}
}

func TestParseScheduleState(t *testing.T) {
	if st, ok := ParseScheduleState(" enabled "); !ok || st != ScheduleEnabled {
		t.Errorf("case-insensitive parse failed: %v %v", st, ok)
	}
	if _, ok := ParseScheduleState("running"); ok {
		t.Error("RUNNING is observed, never settable")
	}
	if _, ok := ParseScheduleState("bogus"); ok {
		t.Error("unknown state accepted")
	}
}
