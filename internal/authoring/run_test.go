package authoring

import (
	"encoding/json"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestRunOptions_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		opts RunOptions
		want string
	}{
		{
			name: "no tunnel sends an explicit null, not an omission",
			opts: RunOptions{BuildName: "nightly"},
			want: `{"buildName":"nightly","scTunnelName":null}`,
		},
		{
			name: "tunnel is sent by name",
			opts: RunOptions{BuildName: "nightly", TunnelName: "my-tunnel"},
			want: `{"buildName":"nightly","scTunnelName":"my-tunnel"}`,
		},
		{
			name: "empty build name is omitted and targets carry capabilities only",
			opts: RunOptions{Targets: []Target{{Capabilities: map[string]any{"browserName": "chrome"}}}},
			want: `{"scTunnelName":null,"targets":[{"capabilities":{"browserName":"chrome"}}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tt.want {
				t.Errorf("got  %s\nwant %s", b, tt.want)
			}
		})
	}
}

func TestRunJob_DecodesMissingSuccessAsNil(t *testing.T) {
	// The start response has no success key at all; that must decode to
	// "not yet reported", never to "failed".
	var j RunJob
	if err := json.Unmarshal([]byte(`{"id":"j","name":"n","sauceJobId":"s","target":{"capabilities":{},"isRdc":false}}`), &j); err != nil {
		t.Fatal(err)
	}
	if j.Success != nil {
		t.Fatalf("expected nil success, got %v", *j.Success)
	}
	if j.Done() || j.Passed() {
		t.Error("a job without an outcome must be neither done nor passed")
	}
}

func TestRun_DoneAndPassed(t *testing.T) {
	tests := []struct {
		name       string
		run        Run
		wantDone   bool
		wantPassed bool
	}{
		{name: "no jobs is not done", run: Run{}},
		{name: "job without outcome", run: Run{Jobs: []RunJob{{ID: "a"}}}},
		{name: "one of two still running", run: Run{Jobs: []RunJob{{Success: boolPtr(true)}, {}}}},
		{name: "error text alone is terminal and failed", run: Run{Jobs: []RunJob{{Error: "boom"}}}, wantDone: true},
		{name: "explicit false", run: Run{Jobs: []RunJob{{Success: boolPtr(false)}}}, wantDone: true},
		{name: "all passed", run: Run{Jobs: []RunJob{{Success: boolPtr(true)}, {Success: boolPtr(true)}}}, wantDone: true, wantPassed: true},
		{name: "mixed", run: Run{Jobs: []RunJob{{Success: boolPtr(true)}, {Success: boolPtr(false)}}}, wantDone: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.run.Done(); got != tt.wantDone {
				t.Errorf("Done() = %v, want %v", got, tt.wantDone)
			}
			if got := tt.run.Passed(); got != tt.wantPassed {
				t.Errorf("Passed() = %v, want %v", got, tt.wantPassed)
			}
		})
	}
}

func TestRunJob_RealDevice(t *testing.T) {
	if (RunJob{}).RealDevice() {
		t.Error("expected virtual by default")
	}
	if !(RunJob{IsRDC: true}).RealDevice() {
		t.Error("job-level flag must count")
	}
	if !(RunJob{Target: Target{IsRDC: true}}).RealDevice() {
		t.Error("target-level flag must count; it is the only one present early in a run")
	}
}
