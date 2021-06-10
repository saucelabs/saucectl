package init

import (
	"bytes"
	"fmt"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/hinshun/vt10x"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

// Test Case setup is partially reused from:
//  - https://github.com/AlecAivazis/survey/blob/master/survey_test.go
//  - https://github.com/AlecAivazis/survey/blob/master/survey_posix_test.go

func executeQuestionTest(t *testing.T, test questionTest) {
	buf := new(bytes.Buffer)
	c, state, err := vt10x.NewVT10XConsole(expect.WithStdout(buf))
	require.Nil(t, err)
	defer c.Close()

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		test.procedure(c)
	}()

	test.ini.stdio = terminal.Stdio{In: c.Tty(), Out: c.Tty(), Err: c.Tty()}
	err = test.execution(test.ini, test.startState)
	require.Nil(t, err)

	if !reflect.DeepEqual(test.startState, test.expectedState) {
		t.Errorf("got: %v, want: %v", test.startState, test.expectedState)
	}

	// Close the slave end of the pty, and read the remaining bytes from the master end.
	c.Tty().Close()
	<-donec

	t.Logf("Raw output: %q", buf.String())

	// Dump the terminal's screen.
	t.Logf("\n%s", expect.StripTrailingEmptyLines(state.String()))
}

func stringToProcedure(actions string) func(*expect.Console) {
	return func(c *expect.Console) {
		for _, chr := range actions {
			switch chr {
			case '↓':
				c.Send(string(terminal.KeyArrowDown))
			case '↑':
				c.Send(string(terminal.KeyArrowUp))
			case '✓':
				c.Send(string(terminal.KeyEnter))
			case '🔚':
				c.ExpectEOF()
			default:
				c.Send(fmt.Sprintf("%c", chr))
			}
		}
	}
}

type questionTest struct {
	name          string
	ini           *initiator
	execution     func(*initiator, *initConfig) error
	procedure     func(*expect.Console)
	startState    *initConfig
	expectedState *initConfig
}

func TestAskFramework(t *testing.T) {
	testCases := []questionTest{
		{
			name: "Default",
			procedure: stringToProcedure("✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askFramework(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{frameworkName: "cypress"},
		},
		{
			name: "Type In",
			procedure: stringToProcedure("esp✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askFramework(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{frameworkName: "espresso"},
		},
		{
			name: "Arrow In",
			procedure: stringToProcedure("↓✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askFramework(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{frameworkName: "espresso"},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTest(lt, tt)
		})
	}
}

func TestAskRegion(t *testing.T) {
	testCases := []questionTest{
		{
			name: "Default",
			procedure: stringToProcedure("✓🔚"),
			ini: &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askRegion(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{region: region.USWest1.String()},
		},
		{
			name: "Type US",
			procedure: stringToProcedure("us-✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askRegion(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{region: region.USWest1.String()},
		},
		{
			name: "Type EU",
			procedure: stringToProcedure("eu-✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askRegion(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{region: region.EUCentral1.String()},
		},
		{
			name: "Select EU",
			procedure: stringToProcedure("↓✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askRegion(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{region: region.EUCentral1.String()},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTest(lt, tt)
		})
	}
}

func TestAskDownloadWhen(t *testing.T) {
	testCases := []questionTest{
		{
			name: "Defaults to Fail",
			procedure: stringToProcedure("✓🔚"),
			ini: &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDownloadWhen(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{artifactWhen: config.WhenFail},
		},
		{
			name: "Second is pass",
			procedure: stringToProcedure("↓✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDownloadWhen(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{artifactWhen: config.WhenPass},
		},
		{
			name: "Type always",
			procedure: stringToProcedure("alw✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDownloadWhen(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{artifactWhen: config.WhenAlways},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTest(lt, tt)
		})
	}
}