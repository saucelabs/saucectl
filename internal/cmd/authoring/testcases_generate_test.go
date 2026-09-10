package authoring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/saucelabs/saucectl/internal/mocks"
)

// step builds a generation step for the fake sequence.
func step(tool authoring.ToolType, args string, success bool) authoring.GenerationStep {
	return authoring.GenerationStep{
		Action: authoring.Tool{Type: tool, Args: json.RawMessage(args)},
		Result: &authoring.StepResult{Success: success},
	}
}

func TestWatchGeneration_RendersEachStepOnce(t *testing.T) {
	steps := []authoring.GenerationStep{
		step(authoring.ToolGoToURL, `{"url":"https://saucedemo.com"}`, true),
		step(authoring.ToolInputText, `{"selector":{"type":"css","value":"#user-name"},"text":"standard_user"}`, true),
		step(authoring.ToolClick, `{"selector":{"type":"css","value":"#login-button"}}`, false),
		step(authoring.ToolFinish, `{}`, true),
	}
	sequence := []authoring.GenerationState{
		{Status: authoring.GenerationQueued},
		{Status: authoring.GenerationInProgress, Steps: steps[:2], Reasoning: []authoring.Reasoning{{Title: "Navigating to the login page"}}},
		{Status: authoring.GenerationInProgress, Steps: steps[:4], Reasoning: []authoring.Reasoning{{Title: "Navigating to the login page"}}},
		{Status: authoring.GenerationCompleted, Steps: steps, TestCaseID: "new-id"},
	}

	calls := 0
	svc := &mocks.AuthoringService{GenerationStatusFn: func(_ context.Context, taskID string) (authoring.GenerationState, error) {
		if taskID != "task" {
			t.Errorf("taskID = %q", taskID)
		}
		s := sequence[calls]
		if calls < len(sequence)-1 {
			calls++
		}
		return s, nil
	}}

	var out bytes.Buffer
	state, err := watchGeneration(context.Background(), svc, "task", time.Millisecond, &out, false)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != authoring.GenerationCompleted || state.TestCaseID != "new-id" {
		t.Errorf("final state = %+v", state)
	}
	if calls != 3 {
		t.Errorf("polled %d times before terminal, want 3", calls+1)
	}

	text := out.String()
	for _, want := range []string{
		"* Navigating to the login page",
		"✓ go_to_url https://saucedemo.com",
		"✓ input_text css=#user-name ← standard_user",
		"✗ click css=#login-button",
		"✓ finish",
	} {
		if strings.Count(text, want) != 1 {
			t.Errorf("%q rendered %d times, want exactly once:\n%s", want, strings.Count(text, want), text)
		}
	}
}

func TestWatchGeneration_PollsBeforeWaiting(t *testing.T) {
	// An already-finished task must return without sleeping through an
	// interval.
	svc := &mocks.AuthoringService{GenerationStatusFn: func(context.Context, string) (authoring.GenerationState, error) {
		return authoring.GenerationState{Status: authoring.GenerationFailed, Error: &authoring.GenerationError{Code: "X", Detail: "boom"}}, nil
	}}
	start := time.Now()
	state, err := watchGeneration(context.Background(), svc, "task", time.Hour, &bytes.Buffer{}, false)
	if err != nil || state.Status != authoring.GenerationFailed {
		t.Fatalf("state = %+v, err = %v", state, err)
	}
	if time.Since(start) > time.Second {
		t.Error("waited for an interval before the first poll")
	}
}

func TestWatchGeneration_CancelledContext(t *testing.T) {
	svc := &mocks.AuthoringService{GenerationStatusFn: func(context.Context, string) (authoring.GenerationState, error) {
		return authoring.GenerationState{Status: authoring.GenerationInProgress}, nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state, err := watchGeneration(ctx, svc, "task", time.Millisecond, &bytes.Buffer{}, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if state.Status != authoring.GenerationInProgress {
		t.Errorf("last state must be returned alongside the error, got %+v", state)
	}
}

func TestWatchGeneration_TransientPollErrorIsRetriedThenReported(t *testing.T) {
	// A plain error could be propagation lag, so it is retried; when it never
	// clears, the wait ends on the cause rather than on a misleading "still
	// running", and the caller's deadline bounds the retrying.
	// The deadline is driven by the fake rather than the wall clock: an
	// earlier version of this test asserted a poll count inside 50ms and
	// failed under parallel load.
	boom := errors.New("boom")
	calls := 0
	deadline, expire := context.WithCancel(context.Background())
	defer expire()
	svc := &mocks.AuthoringService{GenerationStatusFn: func(context.Context, string) (authoring.GenerationState, error) {
		calls++
		if calls == 3 {
			expire()
		}
		return authoring.GenerationState{}, boom
	}}
	// A cancelled context means "the user stopped us", which reports the
	// context error; the deadline case is what reports the poll failure, so
	// wrap the cancellation as a deadline for this assertion.
	ctx := deadlineContext{deadline}
	_, err := watchGeneration(ctx, svc, "task", time.Millisecond, &bytes.Buffer{}, false)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v; a deadline reached while polls keep failing must report the failure", err)
	}
	if calls != 3 {
		t.Errorf("polled %d time(s); a transient error should be retried", calls)
	}
}

func TestWatchGeneration_FatalPollErrorReturnsImmediately(t *testing.T) {
	// A 4xx other than 404 cannot clear by waiting, so it must not be retried.
	forbidden := &authoring.APIError{HTTPStatus: 403, Code: "UNAUTHORIZED"}
	calls := 0
	svc := &mocks.AuthoringService{GenerationStatusFn: func(context.Context, string) (authoring.GenerationState, error) {
		calls++
		return authoring.GenerationState{}, forbidden
	}}
	_, err := watchGeneration(context.Background(), svc, "task", time.Hour, &bytes.Buffer{}, false)
	if !errors.Is(err, forbidden) {
		t.Fatalf("err = %v", err)
	}
	if calls != 1 {
		t.Errorf("polled %d times; a fatal error must not be retried", calls)
	}
}

func TestWatchGeneration_TransientRetryIsBounded(t *testing.T) {
	// Even with an unbounded context the loop must end.
	if maxTransientPollWindow <= 0 || maxTransientPollWindow > 5*time.Minute {
		t.Errorf("maxTransientPollWindow = %v; want a small positive bound", maxTransientPollWindow)
	}
}

func TestFormatGenerationStep(t *testing.T) {
	if got := formatGenerationStep(authoring.GenerationStep{Action: authoring.Tool{Type: authoring.ToolFinish}}); got != "* finish" {
		t.Errorf("no result: %q", got)
	}
	s := step(authoring.ToolClick, `{"selector":{"type":"css","value":"#a"}}`, false)
	s.Result.Message = "element not interactable"
	if got := formatGenerationStep(s); got != "✗ click css=#a  (element not interactable)" {
		t.Errorf("failure: %q", got)
	}
}

// deadlineContext reports cancellation as a deadline, so a test can drive the
// "our own deadline expired" path without waiting on a real clock. It must
// stay indistinguishable from a live context until the cancellation actually
// happens, or the code under test sees an expired context on its first look.
type deadlineContext struct{ context.Context }

func (d deadlineContext) Err() error {
	if d.Context.Err() != nil {
		return context.DeadlineExceeded
	}
	return nil
}
