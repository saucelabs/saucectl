package authoring

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
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
	if len(f.name) > 255 {
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
		if len(tag) > 60 {
			return opts, fmt.Errorf("tag %q exceeds 60 characters", tag)
		}
	}
	if len(f.testURL) > 2048 {
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
	if len(intent) > 20000 {
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
		if out == TextOutput {
			fmt.Fprintf(stdout, "\n%s. Check progress with: saucectl authoring testcases generate-status %s --wait\n", ErrGenerationStillRunning, taskID)
		}
		return fmt.Errorf("%w: %v", ErrGenerationStillRunning, err)
	}

	if out == JSONOutput {
		return renderJSON(struct {
			TaskID string `json:"taskId"`
			authoring.GenerationState
		}{TaskID: taskID, GenerationState: state})
	}

	switch state.Status {
	case authoring.GenerationCompleted:
		fmt.Fprintf(stdout, "\nGeneration completed. New test case: %s\nInspect it with: saucectl authoring testcases get %s --show-steps\n", state.TestCaseID, state.TestCaseID)
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
	renderedSteps, renderedReasoning := 0, 0

	for {
		state, err := svc.GenerationStatus(ctx, taskID)
		if err != nil {
			return last, err
		}
		last = state

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
			return last, ctx.Err()
		case <-time.After(interval):
		}
	}
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
