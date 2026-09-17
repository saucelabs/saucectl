package authoring

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saucelabs/saucectl/internal/authoring"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/mocks"
)

func TestParseTargets(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target.json")
	if err := os.WriteFile(file, []byte(`{"capabilities":{"platformName":"Android"},"isRdc":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := parseTargets(
		[]string{`browserName=chrome,platformName="Windows 11",browserVersion=latest,sauce:options.screenResolution=1920x1080,appium:autoGrantPermissions=true`},
		[]string{`{"browserName":"firefox"}`, "@" + file},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d targets, want 3", len(got))
	}

	kv := got[0].Capabilities
	if kv["browserName"] != "chrome" || kv["platformName"] != "Windows 11" || kv["browserVersion"] != "latest" {
		t.Errorf("kv capabilities = %v", kv)
	}
	if kv["appium:autoGrantPermissions"] != true {
		t.Errorf("boolean literal not coerced: %v", kv["appium:autoGrantPermissions"])
	}
	nested, ok := kv["sauce:options"].(map[string]any)
	if !ok || nested["screenResolution"] != "1920x1080" {
		t.Errorf("dotted key not nested: %v", kv["sauce:options"])
	}
	if got[1].Capabilities["browserName"] != "firefox" || got[1].IsRDC {
		t.Errorf("bare json = %+v", got[1])
	}
	if got[2].Capabilities["platformName"] != "Android" || !got[2].IsRDC {
		t.Errorf("wrapped json from file = %+v", got[2])
	}

	for _, bad := range [][]string{{"nokey"}, {"=value"}, {""}} {
		if _, err := parseTargets(bad, nil); err == nil {
			t.Errorf("expected error for %v", bad)
		}
	}
	if _, err := parseTargets(nil, []string{`{not json`}); err == nil {
		t.Error("expected error for invalid json")
	}
	if _, err := parseTargets(nil, []string{`{}`}); err == nil {
		t.Error("expected error for empty json object")
	}
	if _, err := parseTargets(nil, []string{`{"capabilities":"oops"}`}); err == nil {
		t.Error("expected error for non-object capabilities")
	}
}

func TestDescribeTarget(t *testing.T) {
	tests := []struct {
		caps map[string]any
		want string
	}{
		{map[string]any{"browserName": "chrome", "browserVersion": "latest", "platformName": "Windows 11"}, "chrome latest / Windows 11"},
		{map[string]any{"platformName": "Android", "appium:platformVersion": "16", "appium:deviceName": "Google Pixel 9 Emulator"}, "Google Pixel 9 Emulator / Android 16"},
		{map[string]any{}, "-"},
	}
	for _, tt := range tests {
		if got := describeTarget(authoring.Target{Capabilities: tt.caps}); got != tt.want {
			t.Errorf("describeTarget(%v) = %q, want %q", tt.caps, got, tt.want)
		}
	}
}

func TestResolveValue(t *testing.T) {
	t.Setenv("AUTHORING_TEST_VALUE", "from-env")
	t.Setenv("AUTHORING_EMPTY_VALUE", "")

	dir := t.TempDir()
	file := filepath.Join(dir, "value.txt")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	origTerm, origInteractive, origPrompt := stdinIsTerminal, isInteractive, promptValue
	t.Cleanup(func() { stdinIsTerminal, isInteractive, promptValue = origTerm, origInteractive, origPrompt })
	stdinIsTerminal = func() bool { return false }
	isInteractive = func() bool { return false }

	tests := []struct {
		name    string
		vs      valueSource
		stdin   string
		want    string
		wantErr string
	}{
		{name: "env", vs: valueSource{envName: "AUTHORING_TEST_VALUE"}, want: "from-env"},
		{name: "env empty", vs: valueSource{envName: "AUTHORING_EMPTY_VALUE"}, wantErr: "not set or empty"},
		{name: "env missing", vs: valueSource{envName: "AUTHORING_MISSING_VALUE"}, wantErr: "not set or empty"},
		{name: "file strips one newline", vs: valueSource{file: file}, want: "from-file"},
		{name: "file missing", vs: valueSource{file: filepath.Join(dir, "nope")}, wantErr: "reading value file"},
		{name: "stdin", vs: valueSource{file: "-"}, stdin: "piped\r\n", want: "piped"},
		{name: "stdin keeps inner newlines", vs: valueSource{file: "-"}, stdin: "a\nb\n", want: "a\nb"},
		{name: "literal", vs: valueSource{value: "literal"}, want: "literal"},
		{name: "two sources", vs: valueSource{value: "a", envName: "AUTHORING_TEST_VALUE"}, wantErr: "only one of"},
		{name: "none, non-interactive", vs: valueSource{}, wantErr: "no value given"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveValue(tt.vs, strings.NewReader(tt.stdin))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("stdin refused on a terminal", func(t *testing.T) {
		stdinIsTerminal = func() bool { return true }
		defer func() { stdinIsTerminal = func() bool { return false } }()
		if _, err := resolveValue(valueSource{file: "-"}, strings.NewReader("x")); err == nil {
			t.Error("expected refusal")
		}
	})

	t.Run("none, interactive prompts", func(t *testing.T) {
		isInteractive = func() bool { return true }
		defer func() { isInteractive = func() bool { return false } }()
		var askedSecret bool
		promptValue = func(secret bool) (string, error) { askedSecret = secret; return "typed", nil }
		got, err := resolveValue(valueSource{secret: true}, strings.NewReader(""))
		if err != nil || got != "typed" || !askedSecret {
			t.Errorf("got %q, %v, secret=%v", got, err, askedSecret)
		}
	})
}

func TestConfirmDestructive(t *testing.T) {
	origInteractive, origPrompt := isInteractive, promptConfirm
	t.Cleanup(func() { isInteractive, promptConfirm = origInteractive, origPrompt })

	promptCalls := 0
	promptConfirm = func(string) (bool, error) { promptCalls++; return false, nil }

	isInteractive = func() bool { return false }
	if err := confirmDestructive(true, "x", nil); err != nil {
		t.Errorf("--yes non-interactive: %v", err)
	}
	if err := confirmDestructive(false, "x", nil); !errors.Is(err, ErrConfirmationRequired) {
		t.Errorf("non-interactive without --yes must refuse, got %v", err)
	}
	if promptCalls != 0 {
		t.Error("must never prompt when non-interactive")
	}

	isInteractive = func() bool { return true }
	if err := confirmDestructive(true, "x", nil); err != nil || promptCalls != 0 {
		t.Errorf("--yes interactive must not prompt: %v, calls=%d", err, promptCalls)
	}
	if err := confirmDestructive(false, "x", []string{"a"}); !errors.Is(err, ErrAborted) || promptCalls != 1 {
		t.Errorf("declined prompt: err=%v calls=%d", err, promptCalls)
	}
	promptConfirm = func(string) (bool, error) { return true, nil }
	if err := confirmDestructive(false, "x", nil); err != nil {
		t.Errorf("accepted prompt: %v", err)
	}
}

// changedSet stubs pflag.FlagSet.Changed for the overlay test.
type changedSet map[string]bool

func (c changedSet) Changed(name string) bool { return c[name] }

func TestBuildUpdateScheduleOptions(t *testing.T) {
	five := 5
	current := authoring.TestSchedule{
		Name:         "Nightly",
		State:        authoring.ScheduleState{StateName: authoring.ScheduleEnabled},
		TestSuiteIDs: []string{"s1", "s2"},
		Settings: authoring.ScheduleSettings{
			Cron: "0 2 * * *", Timezone: "Europe/Berlin", RunningUserID: "u", MaxRuns: &five, TunnelName: "old-tunnel", BuildName: "nightly",
		},
	}

	t.Run("cron change re-sends the complete settings", func(t *testing.T) {
		f := scheduleUpdateFlags{scheduleFlags: scheduleFlags{cron: "0 3 * * *"}}
		opts, err := buildUpdateScheduleOptions(changedSet{"cron": true}, f, current)
		if err != nil {
			t.Fatal(err)
		}
		s := opts.Settings
		if s == nil || s.Cron != "0 3 * * *" || s.Timezone != "Europe/Berlin" || s.RunningUserID != "u" || s.MaxRuns == nil || *s.MaxRuns != 5 {
			t.Errorf("settings not fully re-sent: %+v", s)
		}
		// The service replaces the schedule, so the rest of the object must
		// always travel along.
		if opts.Name != "Nightly" || opts.StateName != authoring.ScheduleEnabled || len(opts.TestSuiteIDs) != 2 {
			t.Errorf("complete object not re-sent: %+v", opts)
		}
		if s.TunnelName == nil || *s.TunnelName != "old-tunnel" || s.BuildName == nil || *s.BuildName != "nightly" {
			t.Errorf("nullable fields not carried over: %+v", s)
		}
	})

	t.Run("unset tunnelName sends null", func(t *testing.T) {
		f := scheduleUpdateFlags{unset: []string{"tunnelName", "maxRuns"}}
		opts, err := buildUpdateScheduleOptions(changedSet{}, f, current)
		if err != nil {
			t.Fatal(err)
		}
		if opts.Settings.TunnelName == nil || *opts.Settings.TunnelName != "" {
			t.Errorf("tunnelName = %v, want pointer to empty string (null on the wire)", opts.Settings.TunnelName)
		}
		if opts.Settings.MaxRuns != nil || !opts.Settings.ClearMaxRuns {
			t.Error("--unset maxRuns must send an explicit null; omitting keeps the stored value")
		}
	})

	t.Run("unknown unset field", func(t *testing.T) {
		_, err := buildUpdateScheduleOptions(changedSet{}, scheduleUpdateFlags{unset: []string{"cron"}}, current)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("nothing to update", func(t *testing.T) {
		_, err := buildUpdateScheduleOptions(changedSet{}, scheduleUpdateFlags{}, current)
		if !errors.Is(err, authoring.ErrEmptyUpdate) {
			t.Errorf("got %v", err)
		}
	})

	t.Run("wholesale and incremental suites are exclusive", func(t *testing.T) {
		f := scheduleUpdateFlags{scheduleFlags: scheduleFlags{testSuiteIDs: []string{"a"}}, addTestSuiteIDs: []string{"b"}}
		if _, err := buildUpdateScheduleOptions(changedSet{}, f, current); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("state only still sends the complete object", func(t *testing.T) {
		opts, err := buildUpdateScheduleOptions(noFlagChanges{}, scheduleUpdateFlags{scheduleFlags: scheduleFlags{state: "disabled"}}, current)
		if err != nil || opts.StateName != authoring.ScheduleDisabled || opts.Settings == nil || opts.Name != "Nightly" || len(opts.TestSuiteIDs) != 2 {
			t.Errorf("opts = %+v, err = %v", opts, err)
		}
	})

	t.Run("add and remove suites are applied client-side", func(t *testing.T) {
		f := scheduleUpdateFlags{addTestSuiteIDs: []string{"s3", "s1"}, removeTestSuiteIDs: []string{"s2"}}
		opts, err := buildUpdateScheduleOptions(changedSet{}, f, current)
		if err != nil || strings.Join(opts.TestSuiteIDs, ",") != "s1,s3" {
			t.Errorf("suites = %v, err = %v", opts.TestSuiteIDs, err)
		}
	})

	t.Run("removing every suite is refused", func(t *testing.T) {
		f := scheduleUpdateFlags{removeTestSuiteIDs: []string{"s1", "s2"}}
		if _, err := buildUpdateScheduleOptions(changedSet{}, f, current); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("observed state needs an explicit --state", func(t *testing.T) {
		errored := current
		errored.State = authoring.ScheduleState{StateName: authoring.ScheduleErrored}
		if _, err := buildUpdateScheduleOptions(changedSet{"cron": true}, scheduleUpdateFlags{scheduleFlags: scheduleFlags{cron: "x"}}, errored); err == nil {
			t.Error("expected error asking for --state")
		}
		f := scheduleUpdateFlags{scheduleFlags: scheduleFlags{cron: "x", state: "enabled"}}
		if _, err := buildUpdateScheduleOptions(changedSet{"cron": true}, f, errored); err != nil {
			t.Errorf("explicit state should be accepted: %v", err)
		}
	})
}

func TestBuildCreateScheduleOptions(t *testing.T) {
	opts, err := buildCreateScheduleOptions(scheduleFlags{name: "n", cron: "* * * * *", timezone: "Europe/Berlin", testSuiteIDs: []string{"s"}}, "caller")
	if err != nil {
		t.Fatal(err)
	}
	if opts.Settings.Timezone != "Europe/Berlin" || opts.Settings.RunningUserID != "caller" || opts.StateName != authoring.ScheduleEnabled {
		t.Errorf("defaults not applied: %+v", opts)
	}
	if opts.Settings.MaxRuns != nil {
		t.Error("maxRuns must be nil when not given")
	}
	for _, f := range []scheduleFlags{
		{cron: "* * * * *", timezone: "Europe/Berlin", testSuiteIDs: []string{"s"}},
		{name: "n", timezone: "Europe/Berlin", testSuiteIDs: []string{"s"}},
		{name: "n", cron: "* * * * *", timezone: "Europe/Berlin"},
		{name: "n", cron: "* * * * *", testSuiteIDs: []string{"s"}},
		{name: "n", cron: "* * * * *", timezone: "Europe/Berlin", testSuiteIDs: []string{"s"}, state: "running"},
	} {
		if _, err := buildCreateScheduleOptions(f, "caller"); err == nil {
			t.Errorf("expected error for %+v", f)
		}
	}
}

func TestDeriveFilename(t *testing.T) {
	tests := []struct {
		target, name, code string
		want               string
		known              bool
	}{
		{"typescript_playwright", "Login: Add to cart!", "", "login_add_to_cart.spec.ts", true},
		{"javascript_webdriverio", "Login", "", "login.spec.js", true},
		{"python_selenium", "Checkout Flow", "", "test_checkout_flow.py", true},
		{"csharp_selenium", "checkout flow", "", "CheckoutFlow.cs", true},
		{"java_selenium", "whatever", "package x;\npublic class LoginTest {\n}", "LoginTest.java", true},
		{"java_selenium", "whatever", "public final class FinalOne {}", "FinalOne.java", true},
		{"java_selenium", "login flow", "// no class here", "LoginFlow.java", true},
		{"ruby_capybara", "Login", "", "login_spec.rb", true},
		{"kotlin_appium", "login", "", "Login.kt", true},
		{"cobol_thing", "Login", "", "login.txt", false},
		{"python_selenium", "   ", "", "test_test.py", true},
		{"python_selenium", "123 go", "", "test_test_123_go.py", true},
		{"csharp_selenium", "9lives", "", "Test9lives.cs", true},
	}
	for _, tt := range tests {
		got, known := deriveFilename(tt.target, tt.name, tt.code)
		if got != tt.want || known != tt.known {
			t.Errorf("deriveFilename(%q, %q) = %q, %v; want %q, %v", tt.target, tt.name, got, known, tt.want, tt.known)
		}
	}
}

func TestVerifyEntitlement(t *testing.T) {
	users := &mocks.UserService{UserFn: func(context.Context) (iam.User, error) {
		return iam.User{ID: "u", Organization: iam.Organization{ID: "org"}}, nil
	}}

	t.Run("enabled", func(t *testing.T) {
		ents := &mocks.AuthoringService{IsAIAuthoringEnabledFn: func(_ context.Context, orgID string) (bool, error) {
			if orgID != "org" {
				t.Errorf("orgID = %q", orgID)
			}
			return true, nil
		}}
		u, err := authoring.VerifyEntitlement(context.Background(), users, ents)
		if err != nil || u.ID != "u" {
			t.Errorf("got %+v, %v", u, err)
		}
	})

	t.Run("not in plan", func(t *testing.T) {
		ents := &mocks.AuthoringService{IsAIAuthoringEnabledFn: func(context.Context, string) (bool, error) { return false, nil }}
		_, err := authoring.VerifyEntitlement(context.Background(), users, ents)
		if !errors.Is(err, authoring.ErrNotEntitled) {
			t.Errorf("got %v", err)
		}
	})

	t.Run("could not verify is distinct", func(t *testing.T) {
		ents := &mocks.AuthoringService{IsAIAuthoringEnabledFn: func(context.Context, string) (bool, error) { return false, errors.New("503") }}
		_, err := authoring.VerifyEntitlement(context.Background(), users, ents)
		if err == nil || errors.Is(err, authoring.ErrNotEntitled) || !strings.Contains(err.Error(), "could not verify") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("user lookup failure", func(t *testing.T) {
		badUsers := &mocks.UserService{UserFn: func(context.Context) (iam.User, error) { return iam.User{}, errors.New("401") }}
		ents := &mocks.AuthoringService{IsAIAuthoringEnabledFn: func(context.Context, string) (bool, error) {
			t.Error("entitlement must not be checked without an organisation")
			return true, nil
		}}
		_, err := authoring.VerifyEntitlement(context.Background(), badUsers, ents)
		if err == nil || errors.Is(err, authoring.ErrNotEntitled) {
			t.Errorf("got %v", err)
		}
	})
}

func TestBuildGenerateOptions(t *testing.T) {
	origTerm := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = origTerm })
	stdinIsTerminal = func() bool { return false }

	base := generateFlags{name: "n", intent: "do it", kvTargets: []string{"browserName=chrome"}}

	opts, err := buildGenerateOptions(base, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if opts.PromptSettings.Intent != "do it" || opts.RunSettings.Target.Capabilities["browserName"] != "chrome" || opts.TimeoutMillis != 0 {
		t.Errorf("opts = %+v", opts)
	}

	withTimeout := base
	withTimeout.generationTimeout = 2 * 60 * 1e9
	opts, err = buildGenerateOptions(withTimeout, strings.NewReader(""))
	if err != nil || opts.TimeoutMillis != 120000 {
		t.Errorf("timeout ms = %d, err = %v; the service wants milliseconds", opts.TimeoutMillis, err)
	}

	fromStdin := generateFlags{name: "n", intentFile: "-", kvTargets: []string{"browserName=chrome"}}
	opts, err = buildGenerateOptions(fromStdin, strings.NewReader("  piped intent \n"))
	if err != nil || opts.PromptSettings.Intent != "piped intent" {
		t.Errorf("stdin intent = %q, err = %v", opts.PromptSettings.Intent, err)
	}

	bad := []generateFlags{
		{intent: "x", kvTargets: []string{"browserName=chrome"}},
		{name: "n", kvTargets: []string{"browserName=chrome"}},
		{name: "n", intent: "x", intentFile: "f", kvTargets: []string{"browserName=chrome"}},
		{name: "n", intent: "x"},
		{name: "n", intent: "x", kvTargets: []string{"browserName=chrome", "browserName=firefox"}},
		{name: "n", intent: "x", kvTargets: []string{"browserName=chrome"}, maxSteps: 201},
		{name: "n", intent: "x", kvTargets: []string{"browserName=chrome"}, generationTimeout: 30 * 1e9},
		{name: "n", intent: "x", kvTargets: []string{"browserName=chrome"}, tags: []string{strings.Repeat("x", 61)}},
	}
	for i, f := range bad {
		if _, err := buildGenerateOptions(f, strings.NewReader("")); err == nil {
			t.Errorf("case %d: expected error for %+v", i, f)
		}
	}
}
