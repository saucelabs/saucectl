package init

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/hinshun/vt10x"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/fs"
	"os"
	"reflect"
	"strings"
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
		if lerr := test.procedure(c); lerr != nil {
			t.Errorf("error: %v", lerr)
			t.FailNow()
		}
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

func stringToProcedure(actions string) func(*expect.Console) error {
	return func(c *expect.Console) error {
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
		return nil
	}
}

type questionTest struct {
	name          string
	ini           *initiator
	execution     func(*initiator, *initConfig) error
	procedure     func(*expect.Console) error
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
			ini: &initiator{},
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

func TestAskDevice(t *testing.T) {
	testCases := []questionTest{
		{
			name: "Empty is allowed",
			procedure: stringToProcedure("✓🔚"),
			ini: &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDevice(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{},
		},
		{
			name: "Input is captured",
			procedure: stringToProcedure("Google Pixel✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDevice(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{device: config.Device{Name: "Google Pixel"}},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTest(lt, tt)
		})
	}
}

func TestAskEmulator(t *testing.T) {
	testCases := []questionTest{
		{
			name: "Empty is allowed",
			procedure: func (c *expect.Console) error {
				_, err := c.ExpectString("Type emulator name:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("")
				if err != nil {
					return err
				}
				_, err = c.Send(string(terminal.KeyEnter))
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askEmulator(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{},
		},
		{
			name: "Input is captured",
			procedure:  func (c *expect.Console) error {
				_, err := c.ExpectString("Type emulator name")
				if err != nil {
					return err
				}
				_, err = c.Send("Google Pixel Emulator")
				if err != nil {
					return err
				}
				_, err = c.Send(string(terminal.KeyEnter))
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askEmulator(cfg)
			},
			startState: &initConfig{},
			expectedState: &initConfig{emulator: config.Emulator{Name: "Google Pixel Emulator"}},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTest(lt, tt)
		})
	}
}

func TestAskPlatform(t *testing.T) {
	testCases := []questionTest{
		{
			name: "Windows 10",
			procedure: func (c *expect.Console) error {
				_, err := c.ExpectString("Select platform")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyEnter))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select Browser")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyEnter))
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askPlatform(cfg)
			},
			startState: &initConfig{frameworkName: "testcafe"},
			expectedState: &initConfig{frameworkName: "testcafe", browserName: "chrome", mode: "sauce", platformName: "Windows 10"},
		},
		{
			name: "macOS",
			procedure:  func (c *expect.Console) error {
				_, err := c.ExpectString("Select platform")
				if err != nil {
					return err
				}
				_, err = c.SendLine("macOS")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select Browser")
				if err != nil {
					return err
				}
				_, err = c.SendLine("firefox")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askPlatform(cfg)
			},
			startState: &initConfig{frameworkName: "testcafe"},
			expectedState: &initConfig{frameworkName: "testcafe", platformName: "macOS 11.0", browserName: "firefox", mode: "sauce"},
		},
		{
			name: "docker",
			procedure:  func (c *expect.Console) error {
				_, err := c.ExpectString("Select platform")
				if err != nil {
					return err
				}
				_, err = c.SendLine("docker")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select Browser")
				if err != nil {
					return err
				}
				_, err = c.SendLine("chrome")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askPlatform(cfg)
			},
			startState: &initConfig{frameworkName: "testcafe"},
			expectedState: &initConfig{frameworkName: "testcafe", platformName: "", browserName: "chrome", mode: "docker"},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTest(lt, tt)
		})
	}
}

func TestAskVersion(t *testing.T) {
	testCases := []questionTest{
		{
			name: "Default",
			procedure: func (c *expect.Console) error {
				_, err := c.ExpectString("Select cypress version")
				if err != nil {
					return err
				}
				_, err = c.SendLine(string(terminal.KeyEnter))
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askVersion(cfg)
			},
			startState: &initConfig{frameworkName: "cypress"},
			expectedState: &initConfig{frameworkName: "cypress", frameworkVersion: "7.6.0"},
		},
		{
			name: "Second",
			procedure:  func (c *expect.Console) error {
				_, err := c.ExpectString("Select cypress version")
				if err != nil {
					return err
				}
				_, err = c.Send(string(terminal.KeyArrowDown))
				if err != nil {
					return err
				}
				_, err = c.Send(string(terminal.KeyEnter))
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askVersion(cfg)
			},
			startState: &initConfig{frameworkName: "cypress"},
			expectedState: &initConfig{frameworkName: "cypress", frameworkVersion: "7.5.0"},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTest(lt, tt)
		})
	}
}

func TestAskFile(t *testing.T) {
	dir := fs.NewDir(t, "apps",
		fs.WithFile("android-app.apk", "myAppContent", fs.WithMode(0644)),
		fs.WithFile("ios-app.ipa", "myAppContent", fs.WithMode(0644)),
		fs.WithDir("ios-folder-app.app", fs.WithMode(0755)))
	defer dir.Remove()

	testCases := []questionTest{
		{
			name: "Default",
			procedure: func (c *expect.Console) error {
				_, err := c.ExpectString("Filename")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("android"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Sorry, your reply was invalid")
				if err != nil {
					return err
				}
				_, err = c.SendLine("-app.apk")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askFile("Filename", func(ans interface{}) error {
					val := ans.(string)
					if !strings.HasSuffix(val, ".apk") {
						return errors.New("not-an-apk")
					}
					fi, err := os.Stat(val)
					if err != nil {
						return err
					}
					if fi.IsDir() {
						return errors.New("not-a-file")
					}
					return nil
				}, nil, &cfg.app)
			},
			startState: &initConfig{},
			expectedState: &initConfig{app: dir.Join("android-app.apk")},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTest(lt, tt)
		})
	}
}