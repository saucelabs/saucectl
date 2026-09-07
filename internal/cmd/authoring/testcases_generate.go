package authoring

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/briandowns/spinner"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// Generation timing defaults. The service recommends polling every 2–3 s
// while a task is queued or in progress. defaultWaitTimeout is what bounds
// the wait when neither --wait-timeout nor --generation-timeout is given: the
// service's own one-hour maximum plus a margin.
const (
	defaultGenerationPoll = 3 * time.Second
	waitTimeoutMargin     = 2 * time.Minute
	defaultWaitTimeout    = time.Hour + waitTimeoutMargin
	minGenerationTimeout  = time.Minute
	maxGenerationTimeout  = time.Hour
	// maxTransientPollWindow bounds how long consecutive transient poll
	// failures are tolerated before the last one is reported. Propagation
	// lag lasts seconds; a minute of unbroken failure is the service telling
	// us something. Bounding it here rather than relying on the caller's
	// deadline keeps the promise that nothing waits for ever
	// (Constitution VIII) even if a caller forgets to set one.
	maxTransientPollWindow = time.Minute
)

// ErrGenerationStillRunning is returned when the wait ends (interrupt or
// timeout) before the task does. The command prints how to reattach.
var ErrGenerationStillRunning = errors.New("generation is still running on Sauce Labs")

// generateFlags are the flags of `testcases generate`.
type generateFlags struct {
	out               string
	name              string
	intent            string
	intentFile        string
	maxSteps          int
	testSuiteID       string
	tags              []string
	kvTargets         []string
	jsonTargets       []string
	testURL           string
	tunnelName        string
	generationTimeout time.Duration
	wait              bool
	pollInterval      time.Duration
	waitTimeout       time.Duration
}

// TestCasesGenerateCommand is `authoring testcases generate`: author a test
// case from a plain-language description.
func TestCasesGenerateCommand() *cobra.Command {
	var f generateFlags

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Author a new test case from a plain-language description",
		Long: `Author a new test case: describe the journey, name the starting URL and a target, and an
AI agent drives a real browser to work out the steps, saving the result as a test case.

Authoring is asynchronous. Without --wait the command prints the task ID and returns; with
--wait it streams each attempted action as it happens. An interrupted or timed-out wait
leaves the task running on Sauce Labs; reattach with 'testcases generate-status <task-id>'.

Three timeouts apply. --generation-timeout is the budget given to the service (1m–1h).
--wait-timeout bounds how long this command watches; when 0 it derives from
--generation-timeout plus two minutes, or defaults to 1h2m. Individual requests have
their own short timeout.`,
		Example: `  saucectl authoring testcases generate --name "Login" \
      --intent "Log in as standard_user and verify the inventory page loads" \
      --test-url https://www.saucedemo.com --target browserName=chrome,platformName="Windows 11" --wait
  saucectl authoring testcases generate --name "Checkout" --intent-file checkout.txt \
      --target-json @pixel9.json --tag mobile --generation-timeout 15m`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGenerate(cmd.Context(), f, os.Stdin, os.Stdout)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&f.out, "out", "o", TextOutput, "Output format. Options: text, json. With json, progress is not streamed; one final object is printed.")
	flags.StringVar(&f.name, "name", "", "Name of the new test case (1–255 characters). Required.")
	flags.StringVar(&f.intent, "intent", "", "Plain-language description of what the test should do (1–20000 characters).")
	flags.StringVar(&f.intentFile, "intent-file", "", "Read the description from a file, or - for standard input.")
	flags.IntVar(&f.maxSteps, "max-steps", 0, "Maximum number of actions the agent may take (1–200). Default: service default.")
	flags.StringVar(&f.testSuiteID, "test-suite-id", "", "Assign the new test case to this suite.")
	flags.StringArrayVar(&f.tags, "tag", nil, "Tag for the new test case (at most 20, each at most 60 characters). Repeatable.")
	flags.StringArrayVar(&f.kvTargets, "target", nil, "Target capabilities as key=value pairs, e.g. browserName=chrome,platformName=\"Windows 11\".")
	flags.StringArrayVar(&f.jsonTargets, "target-json", nil, "Target capabilities as a JSON object, or @path to a file.")
	flags.StringVar(&f.testURL, "test-url", "", "Starting URL (at most 2048 characters).")
	flags.StringVar(&f.tunnelName, "tunnel-name", "", "Name of an active Sauce Connect tunnel to route the session through.")
	flags.DurationVar(&f.generationTimeout, "generation-timeout", 0, "Service-side authoring budget, 1m to 1h. Default: service default.")
	flags.BoolVar(&f.wait, "wait", false, "Wait for authoring to finish, streaming each action as it happens.")
	flags.DurationVar(&f.pollInterval, "poll-interval", defaultGenerationPoll, "How often to poll while waiting.")
	flags.DurationVar(&f.waitTimeout, "wait-timeout", 0, "How long to wait before giving up locally. 0 derives it from --generation-timeout.")

	return cmd
}

// runGenerate validates the flags, starts the task and optionally waits.
func runGenerate(ctx context.Context, f generateFlags, stdin io.Reader, stdout io.Writer) error {
	if err := validateOutput(f.out); err != nil {
		return err
	}
	opts, err := buildGenerateOptions(f, stdin)
	if err != nil {
		return err
	}

	task, err := testCaseService.Generate(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to start generation: %w", err)
	}

	if !f.wait {
		if f.out == JSONOutput {
			return renderJSON(task)
		}
		fmt.Fprintf(stdout, "Generation task accepted.\n  Task ID:       %s\n  Sauce job ID:  %s\n\nCheck progress with: saucectl authoring testcases generate-status %s --wait\n", task.TaskID, task.SauceJobID, task.TaskID)
		return nil
	}

	if f.out == TextOutput {
		fmt.Fprintf(stdout, "Generation task accepted.\n  Task ID:       %s\n  Sauce job ID:  %s\n\n", task.TaskID, task.SauceJobID)
	}

	waitTimeout := f.waitTimeout
	if waitTimeout <= 0 {
		waitTimeout = defaultWaitTimeout
		if f.generationTimeout > 0 {
			waitTimeout = f.generationTimeout + waitTimeoutMargin
		}
	}
	return waitForGeneration(ctx, task.TaskID, f.pollInterval, waitTimeout, f.out, stdout)
}

// buildGenerateOptions turns flags into the request, validating the bounds
// the service enforces so the user gets a precise message instead of
// INVALID_BODY.
func buildGenerateOptions(f generateFlags, stdin io.Reader) (authoring.GenerateOptions, error) {
	var opts authoring.GenerateOptions

	if strings.TrimSpace(f.name) == "" {
		return opts, errors.New("--name is required")
	}
	if utf8.RuneCountInString(f.name) > 255 {
		return opts, errors.New("--name must be at most 255 characters")
	}

	intent, err := readIntent(f.intent, f.intentFile, stdin)
	if err != nil {
		return opts, err
	}

	if f.maxSteps < 0 || f.maxSteps > 200 {
		return opts, errors.New("--max-steps must be between 1 and 200")
	}
	if len(f.tags) > 20 {
		return opts, errors.New("at most 20 tags are allowed")
	}
	for _, tag := range f.tags {
		if utf8.RuneCountInString(tag) > 60 {
			return opts, fmt.Errorf("tag %q exceeds 60 characters", tag)
		}
	}
	if utf8.RuneCountInString(f.testURL) > 2048 {
		return opts, errors.New("--test-url must be at most 2048 characters")
	}
	if f.generationTimeout != 0 && (f.generationTimeout < minGenerationTimeout || f.generationTimeout > maxGenerationTimeout) {
		return opts, errors.New("--generation-timeout must be between 1m and 1h")
	}

	targets, err := parseTargets(f.kvTargets, f.jsonTargets)
	if err != nil {
		return opts, err
	}
	if len(targets) == 0 {
		return opts, ErrNoTargets
	}
	if len(targets) > 1 {
		return opts, errors.New("authoring accepts exactly one target")
	}

	opts = authoring.GenerateOptions{
		Name:        f.name,
		TestSuiteID: f.testSuiteID,
		Tags:        f.tags,
		RunSettings: authoring.GenerateRunSettings{
			Target:     authoring.Target{Capabilities: targets[0].Capabilities},
			TestURL:    f.testURL,
			TunnelName: f.tunnelName,
		},
		PromptSettings: authoring.PromptSettings{
			Intent:   intent,
			MaxSteps: f.maxSteps,
		},
		TimeoutMillis: int(f.generationTimeout / time.Millisecond),
	}
	return opts, nil
}

// readIntent resolves --intent / --intent-file, which are mutually exclusive.
func readIntent(intent, intentFile string, stdin io.Reader) (string, error) {
	if intent != "" && intentFile != "" {
		return "", errors.New("--intent and --intent-file are mutually exclusive")
	}
	if intentFile != "" {
		var b []byte
		var err error
		if intentFile == "-" {
			if stdinIsTerminal() {
				return "", errors.New("--intent-file - reads standard input, but standard input is a terminal")
			}
			b, err = io.ReadAll(stdin)
		} else {
			b, err = os.ReadFile(intentFile)
		}
		if err != nil {
			return "", fmt.Errorf("reading intent: %w", err)
		}
		intent = strings.TrimSpace(string(b))
	}
	if intent == "" {
		return "", errors.New("an intent is required: use --intent or --intent-file")
	}
	if utf8.RuneCountInString(intent) > 20000 {
		return "", errors.New("the intent must be at most 20000 characters")
	}
	return intent, nil
}

// waitForGeneration watches a task until it ends or the wait is bounded,
// then reports the outcome. It never calls os.Exit: a failure is an error the
// root command turns into a non-zero status (FR-029).
func waitForGeneration(ctx context.Context, taskID string, interval, timeout time.Duration, out string, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	render := io.Writer(stdout)
	if out == JSONOutput {
		render = io.Discard
	}

	state, err := watchGeneration(ctx, testCaseService, taskID, interval, render, isTerm(os.Stdout.Fd()) && out == TextOutput)
	if err != nil {
		// Only an abandoned wait means the task is still running. A fatal
		// error — the task does not exist, the credentials no longer work —
		// must not tell the user to keep polling. Either way the cause is
		// wrapped with %w so callers can match the service's sentinels.
		if isAbandonedWait(err) {
			// Whatever the format, the caller needs the task id to reattach:
			// in text it is the printed hint, in JSON it is the emitted
			// object, and it is in the error either way.
			if out == TextOutput {
				fmt.Fprintf(stdout, "\n%s. Check progress with: saucectl authoring testcases generate-status %s --wait\n", ErrGenerationStillRunning, taskID)
			} else {
				_ = renderGenerationJSON(taskID, state)
			}
			return fmt.Errorf("%w: task %s: %w", ErrGenerationStillRunning, taskID, err)
		}
		return fmt.Errorf("failed to follow generation task %s: %w", taskID, err)
	}

	// The payload comes first so a script has it either way, but the task's
	// own status decides the exit code in both formats: returning nil here
	// for a FAILED task made `--wait -o json` exit 0 and broke CI gating.
	if out == JSONOutput {
		if err := renderGenerationJSON(taskID, state); err != nil {
			return err
		}
	}

	switch state.Status {
	case authoring.GenerationCompleted:
		if out == TextOutput {
			fmt.Fprintf(stdout, "\nGeneration completed. New test case: %s\nInspect it with: saucectl authoring testcases get %s --show-steps\n", state.TestCaseID, state.TestCaseID)
		}
		return nil
	case authoring.GenerationFailed:
		if state.Error != nil {
			return *state.Error
		}
		return errors.New("generation failed")
	default:
		return fmt.Errorf("generation ended in unexpected status %q", state.Status)
	}
}

// renderGenerationJSON emits the one final object a JSON-mode wait produces.
// It always carries the task id, which is what makes an abandoned wait
// reattachable without parsing prose.
func renderGenerationJSON(taskID string, state authoring.GenerationState) error {
	return renderJSON(struct {
		TaskID string `json:"taskId"`
		authoring.GenerationState
	}{TaskID: taskID, GenerationState: state})
}

// isAbandonedWait reports whether the wait ended because we stopped waiting
// (interrupt or the local --wait-timeout) rather than because the task or the
// service told us something. Only in that case is the task still running.
func isAbandonedWait(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// watchGeneration polls the task until it reaches a terminal status, the
// context ends, or a poll fails. It polls first and waits after, so an
// already-finished task returns immediately. Each step is rendered exactly
// once as it appears; the spinner is the only difference between interactive
// and non-interactive output. It returns the last state seen alongside any
// error, so callers can still report progress after an interruption.
func watchGeneration(ctx context.Context, svc authoring.TestCaseService, taskID string, interval time.Duration, out io.Writer, spin bool) (authoring.GenerationState, error) {
	if interval <= 0 {
		interval = defaultGenerationPoll
	}

	var sp *spinner.Spinner
	if spin {
		sp = spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(out))
		sp.Suffix = " waiting for the agent..."
		sp.Start()
		defer sp.Stop()
	}

	var last authoring.GenerationState
	// lastErr remembers the most recent transient poll failure. If the wait
	// then ends on its deadline, that error is the honest explanation rather
	// than "still running": a 404 tolerated as propagation lag for one tick
	// is a mistyped task id once it has persisted to the deadline.
	var lastErr error
	var firstTransientAt time.Time
	renderedSteps, renderedReasoning := 0, 0

	for {
		state, err := svc.GenerationStatus(ctx, taskID)
		switch {
		case err == nil:
			last, lastErr, firstTransientAt = state, nil, time.Time{}
		case ctx.Err() != nil:
			// The wait ended at the same moment this poll failed. Route it
			// through the same decision as every other exit so a deadline
			// reached while polls were failing still reports the failure.
			if err != nil {
				lastErr = err
			}
			return last, pollDeadlineError(ctx, lastErr)
		case authoring.IsFatalPollError(err):
			return last, err
		default:
			// A freshly accepted task can 404 until its record propagates,
			// and a truncated body fails to decode; both clear on the next
			// tick. The wait is still bounded by ctx, so tolerating them
			// costs nothing and matches how the runner polls a run.
			lastErr = err
			if firstTransientAt.IsZero() {
				firstTransientAt = time.Now()
			} else if time.Since(firstTransientAt) > maxTransientPollWindow {
				return last, err
			}
			log.Debug().Err(err).Str("task", taskID).Msg("Transient error while polling generation task; retrying.")
			select {
			case <-ctx.Done():
				return last, pollDeadlineError(ctx, lastErr)
			case <-time.After(interval):
			}
			continue
		}

		if sp != nil {
			sp.Stop()
		}
		for ; renderedReasoning < len(state.Reasoning); renderedReasoning++ {
			r := state.Reasoning[renderedReasoning]
			fmt.Fprintf(out, "  * %s\n", r.Title)
		}
		for ; renderedSteps < len(state.Steps); renderedSteps++ {
			fmt.Fprintf(out, "  %s\n", formatGenerationStep(state.Steps[renderedSteps]))
		}
		if sp != nil && !state.Done() {
			sp.Suffix = fmt.Sprintf(" %s, %d step(s) so far...", strings.ToLower(strings.ReplaceAll(string(state.Status), "_", " ")), len(state.Steps))
			sp.Start()
		}

		if state.Done() {
			return state, nil
		}

		select {
		case <-ctx.Done():
			return last, pollDeadlineError(ctx, lastErr)
		case <-time.After(interval):
		}
	}
}

// pollDeadlineError decides what a finished wait actually failed on.
//
// If the user interrupted us, that is the truth regardless of what the last
// poll did: they chose to stop, the task is most likely still running, and
// the reattach hint is what they need. If instead our own deadline expired
// while every poll was failing, that failure is the truth — reporting "still
// running" there would send someone back to re-poll something broken, which
// is the defect this whole path was reported for. A successful last poll
// always means the task really is still going.
func pollDeadlineError(ctx context.Context, lastErr error) error {
	if lastErr != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return lastErr
	}
	return ctx.Err()
}

// formatGenerationStep renders one attempted action with its outcome mark.
func formatGenerationStep(s authoring.GenerationStep) string {
	mark := "*"
	suffix := ""
	if s.Result != nil {
		if s.Result.Success {
			mark = "✓"
		} else {
			mark = "✗"
			if s.Result.Message != "" {
				suffix = "  (" + s.Result.Message + ")"
			}
		}
	}
	return fmt.Sprintf("%s %s%s", mark, s.Action.Summary(), suffix)
}
