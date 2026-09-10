package authoring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/saucelabs/saucectl/internal/mocks"
)

func TestFetchPage_AllHonoursSkip(t *testing.T) {
	// --all used to restart from offset 0, so `--all --skip 100` silently
	// returned the whole listing from the beginning.
	var offsets []int
	fetch := func(_ context.Context, opts authoring.ListOptions) (authoring.List[int], error) {
		offsets = append(offsets, opts.Skip)
		if opts.Skip >= 150 {
			return authoring.List[int]{Items: nil, Total: 150}, nil
		}
		items := make([]int, authoring.DefaultPageSize)
		return authoring.List[int]{Items: items, Total: 150}, nil
	}
	p := pageFlags{all: true, skip: 50, limit: 20}
	if _, _, err := fetchPage(context.Background(), p, "things", fetch); err != nil {
		t.Fatal(err)
	}
	if len(offsets) == 0 || offsets[0] != 50 {
		t.Errorf("first page fetched at offset %v, want the requested skip of 50", offsets)
	}
}

func TestFetchPage_WarnsWhenAllCannotHonourLimit(t *testing.T) {
	// Constitution VIII: a setting the tool cannot honour warns rather than
	// being dropped in silence. Assert the flag state is what drives it.
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	p := &pageFlags{}
	p.bind(fs)
	if err := fs.Parse([]string{"--all", "--limit", "5"}); err != nil {
		t.Fatal(err)
	}
	p.capture(fs)
	if !p.limitChanged {
		t.Error("an explicitly set --limit must be recorded so the warning can fire")
	}

	fs2 := pflag.NewFlagSet("t2", pflag.ContinueOnError)
	p2 := &pageFlags{}
	p2.bind(fs2)
	_ = fs2.Parse([]string{"--all"})
	p2.capture(fs2)
	if p2.limitChanged {
		t.Error("an untouched --limit must not warn")
	}
}

func TestGetTestCase_JSONHonoursRevision(t *testing.T) {
	// --revision was resolved and then discarded under -o json, so a script
	// reading .revisions[-1] got the latest revision instead of the pinned one.
	tc := authoring.TestCase{ID: "tc", Name: "n", Revisions: []authoring.Revision{
		{ID: "old", Intent: "first"},
		{ID: "new", Intent: "second"},
	}}
	testCaseService = &mocks.AuthoringService{
		GetTestCaseFn: func(context.Context, string) (authoring.TestCase, error) { return tc, nil },
	}
	t.Cleanup(func() { testCaseService = nil })

	out := captureStdout(t, func() {
		if err := getTestCase(context.Background(), JSONOutput, "tc", "old", false); err != nil {
			t.Fatal(err)
		}
	})
	var got authoring.TestCase
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(got.Revisions) != 1 || got.Revisions[0].ID != "old" {
		t.Errorf("JSON carried %d revision(s) %v; want only the pinned one", len(got.Revisions), revisionIDs(got))
	}
}

func revisionIDs(tc authoring.TestCase) []string {
	ids := make([]string, 0, len(tc.Revisions))
	for _, r := range tc.Revisions {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestIsAbandonedWait(t *testing.T) {
	// Only our own giving up means "still running"; anything the service
	// said is a real error and must not advise more polling.
	if !isAbandonedWait(context.Canceled) || !isAbandonedWait(context.DeadlineExceeded) {
		t.Error("interrupt and local timeout are abandoned waits")
	}
	if isAbandonedWait(&authoring.APIError{HTTPStatus: 404, Code: "TEST_CASE_GENERATION_TASK_NOT_FOUND"}) {
		t.Error("a 404 from the service is not an abandoned wait")
	}
}

func TestWaitForGeneration_FatalErrorIsNotReportedAsStillRunning(t *testing.T) {
	// A mistyped task id used to print "generation is still running on Sauce
	// Labs. Check progress with: ..." and swallow the cause.
	notFound := &authoring.APIError{HTTPStatus: 404, Code: "TEST_CASE_GENERATION_TASK_NOT_FOUND", Detail: "Test case generation task not found."}
	testCaseService = &mocks.AuthoringService{
		GenerationStatusFn: func(context.Context, string) (authoring.GenerationState, error) {
			return authoring.GenerationState{}, notFound
		},
	}
	t.Cleanup(func() { testCaseService = nil })

	var buf bytes.Buffer
	err := waitForGeneration(context.Background(), "nope", time.Millisecond, time.Second, TextOutput, &buf)
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrGenerationStillRunning) {
		t.Error("a 404 must not be reported as still running")
	}
	if !errors.Is(err, notFound) {
		t.Error("the cause must be wrapped with %w so callers can match the service sentinel")
	}
	if strings.Contains(buf.String(), "still running") {
		t.Errorf("stdout advised more polling for a task that does not exist:\n%s", buf.String())
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	fn()
	w.Close()
	os.Stdout = orig
	return <-done
}

func TestWaitForGeneration_FailedTaskExitsNonZeroInJSONMode(t *testing.T) {
	// The JSON branch used to return before the status switch, so a FAILED
	// task printed its payload and exited 0, breaking CI gating with -o json.
	testCaseService = &mocks.AuthoringService{
		GenerationStatusFn: func(context.Context, string) (authoring.GenerationState, error) {
			return authoring.GenerationState{
				Status: authoring.GenerationFailed,
				Error:  &authoring.GenerationError{Code: "TEST_CASE_EMPTY", Detail: "nothing was recorded"},
			}, nil
		},
	}
	t.Cleanup(func() { testCaseService = nil })

	var err error
	out := captureStdout(t, func() {
		err = waitForGeneration(context.Background(), "task", time.Millisecond, time.Second, JSONOutput, io.Discard)
	})
	if err == nil {
		t.Fatal("a FAILED task must return an error so the process exits non-zero")
	}
	if !strings.Contains(err.Error(), "TEST_CASE_EMPTY") {
		t.Errorf("error lost the service's code: %v", err)
	}
	// The payload still has to reach a script that asked for JSON.
	var doc map[string]any
	if e := json.Unmarshal([]byte(out), &doc); e != nil {
		t.Fatalf("no JSON object was printed: %v\n%s", e, out)
	}
	if doc["status"] != "FAILED" || doc["taskId"] != "task" {
		t.Errorf("JSON = %v; want the failed status and the task id", doc)
	}
}

func TestWaitForGeneration_TimeoutSurfacesTaskIDInBothFormats(t *testing.T) {
	// Without the id there is nothing to reattach with.
	inProgress := func(context.Context, string) (authoring.GenerationState, error) {
		return authoring.GenerationState{Status: authoring.GenerationInProgress}, nil
	}
	testCaseService = &mocks.AuthoringService{GenerationStatusFn: inProgress}
	t.Cleanup(func() { testCaseService = nil })

	var buf bytes.Buffer
	err := waitForGeneration(context.Background(), "task-42", time.Millisecond, 30*time.Millisecond, TextOutput, &buf)
	if err == nil || !strings.Contains(err.Error(), "task-42") {
		t.Errorf("text mode error = %v; want the task id", err)
	}
	if !strings.Contains(buf.String(), "task-42") {
		t.Errorf("text mode printed no reattach hint:\n%s", buf.String())
	}

	out := captureStdout(t, func() {
		err = waitForGeneration(context.Background(), "task-42", time.Millisecond, 30*time.Millisecond, JSONOutput, io.Discard)
	})
	if err == nil || !strings.Contains(err.Error(), "task-42") {
		t.Errorf("json mode error = %v; want the task id", err)
	}
	var doc map[string]any
	if e := json.Unmarshal([]byte(out), &doc); e != nil || doc["taskId"] != "task-42" {
		t.Errorf("json mode emitted %q; want an object carrying taskId", out)
	}
}

func TestBuildCreateScheduleOptions_RejectsUTCLocally(t *testing.T) {
	// The service's accepted list is region/city zones only; catching these
	// two saves a round trip and an opaque INVALID_BODY.
	for _, tz := range []string{"UTC", "utc", "Etc/UTC"} {
		_, err := buildCreateScheduleOptions(scheduleFlags{name: "n", cron: "0 0 3 * * *", timezone: tz, testSuiteIDs: []string{"s"}}, "me")
		if err == nil {
			t.Errorf("--timezone %q was accepted", tz)
			continue
		}
		if !strings.Contains(err.Error(), "Atlantic/Reykjavik") {
			t.Errorf("--timezone %q: error should name a usable zero-offset zone, got %v", tz, err)
		}
	}
	if _, err := buildCreateScheduleOptions(scheduleFlags{name: "n", cron: "0 0 3 * * *", timezone: "Europe/Berlin", testSuiteIDs: []string{"s"}}, "me"); err != nil {
		t.Errorf("a region/city zone must be accepted: %v", err)
	}
}

func TestBuildUpdateScheduleOptions_SetAndUnsetConflict(t *testing.T) {
	// The unset loop ran last and silently won.
	current := authoring.TestSchedule{
		Name:         "n",
		State:        authoring.ScheduleState{StateName: authoring.ScheduleEnabled},
		TestSuiteIDs: []string{"s1"},
		Settings:     authoring.ScheduleSettings{Cron: "c", Timezone: "Europe/Berlin", RunningUserID: "u"},
	}
	cases := []struct{ flag, field string }{
		{"tunnel-name", "tunnelName"},
		{"build", "buildName"},
		{"start-date", "startDate"},
		{"end-date", "endDate"},
		{"max-runs", "maxRuns"},
	}
	for _, c := range cases {
		f := scheduleUpdateFlags{unset: []string{c.field}}
		_, err := buildUpdateScheduleOptions(changedSet{c.flag: true}, f, current)
		if err == nil {
			t.Errorf("--%s with --unset %s was accepted", c.flag, c.field)
			continue
		}
		if !strings.Contains(err.Error(), c.flag) || !strings.Contains(err.Error(), c.field) {
			t.Errorf("--%s/--unset %s: error should name both, got %v", c.flag, c.field, err)
		}
	}
	// Unsetting a field nobody set is still fine.
	if _, err := buildUpdateScheduleOptions(changedSet{}, scheduleUpdateFlags{unset: []string{"buildName"}}, current); err != nil {
		t.Errorf("unset alone must work: %v", err)
	}
}

func TestFetchPage_WarningsAreSuppressedUnderJSON(t *testing.T) {
	// The logger writes to stdout for every command, so a warning emitted
	// while rendering JSON lands inside the document and breaks `| jq`.
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.StringP("out", "o", TextOutput, "")
	p := &pageFlags{}
	p.bind(fs)
	if err := fs.Parse([]string{"--all", "--limit", "5", "-o", "json"}); err != nil {
		t.Fatal(err)
	}
	p.capture(fs)
	if !p.jsonOut {
		t.Error("JSON output must be recorded so advisory warnings can be held back")
	}

	fs2 := pflag.NewFlagSet("t2", pflag.ContinueOnError)
	fs2.StringP("out", "o", TextOutput, "")
	p2 := &pageFlags{}
	p2.bind(fs2)
	_ = fs2.Parse([]string{"--all", "--limit", "5"})
	p2.capture(fs2)
	if p2.jsonOut {
		t.Error("text output must still warn")
	}
}
