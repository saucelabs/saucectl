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
