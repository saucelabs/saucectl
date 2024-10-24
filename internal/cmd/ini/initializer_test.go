package ini

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/saucelabs/saucectl/internal/flags"
	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/imagerunner"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/spf13/pflag"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/fs"

	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"github.com/saucelabs/saucectl/internal/vmd"
	"github.com/saucelabs/saucectl/internal/xcuitest"
)

// Test Case setup is partially reused from:
//  - https://github.com/AlecAivazis/survey/blob/master/survey_test.go
//  - https://github.com/AlecAivazis/survey/blob/master/survey_posix_test.go

// As everything related to console may result in hanging, it's preferable
// to add a timeout to avoid any test to stay for ages.
func executeQuestionTestWithTimeout(t *testing.T, test questionTest) {
	timeout := time.After(2 * time.Second)
	done := make(chan bool)

	go func() {
		executeQuestionTest(t, test)
		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("Test timed-out")
	case <-done:
	}
}

func executeQuestionTest(t *testing.T, test questionTest) {
	buf := new(bytes.Buffer)
	c, state, err := vt10x.NewVT10XConsole(expect.WithStdout(buf))
	require.Nil(t, err)
	defer c.Close()

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		if lerr := test.procedure(c); lerr != nil {
			if lerr.Error() != "read /dev/ptmx: input/output error" {
				t.Errorf("error: %v", lerr)
			}
		}
	}()

	test.ini.stdio = terminal.Stdio{In: c.Tty(), Out: c.Tty(), Err: c.Tty()}
	err = test.execution(test.ini, test.ini.cfg)
	require.Nil(t, err)

	if !reflect.DeepEqual(test.ini.cfg, test.expectedState) {
		t.Errorf("got: %v, want: %v", test.ini.cfg, test.expectedState)
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
			var err error
			switch chr {
			case '↓':
				_, err = c.Send(string(terminal.KeyArrowDown))
			case '↑':
				_, err = c.Send(string(terminal.KeyArrowUp))
			case '✓':
				_, err = c.Send(string(terminal.KeyEnter))
			case '🔚':
				_, err = c.ExpectEOF()
			default:
				_, err = c.Send(fmt.Sprintf("%c", chr))
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
}

type questionTest struct {
	name          string
	ini           *initializer
	execution     func(*initializer, *initConfig) error
	procedure     func(*expect.Console) error
	expectedState *initConfig
}

func TestAskRegion(t *testing.T) {
	testCases := []questionTest{
		{
			name:      "Default",
			procedure: stringToProcedure("✓🔚"),
			ini: &initializer{
				cfg: &initConfig{},
			},
			execution: func(i *initializer, cfg *initConfig) error {
				regio, err := askRegion(i.stdio)
				cfg.region = regio
				return err
			},
			expectedState: &initConfig{region: region.USWest1.String()},
		},
		{
			name:      "Type US",
			procedure: stringToProcedure("us-✓🔚"),
			ini: &initializer{
				cfg: &initConfig{},
			},
			execution: func(i *initializer, cfg *initConfig) error {
				regio, err := askRegion(i.stdio)
				cfg.region = regio
				return err
			},
			expectedState: &initConfig{region: region.USWest1.String()},
		},
		{
			name:      "Type EU",
			procedure: stringToProcedure("eu-✓🔚"),
			ini: &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{},
				cfg:        &initConfig{},
			},
			execution: func(i *initializer, cfg *initConfig) error {
				regio, err := askRegion(i.stdio)
				cfg.region = regio
				return err
			},
			expectedState: &initConfig{region: region.EUCentral1.String()},
		},
		{
			name:      "Select EU",
			procedure: stringToProcedure("↓✓🔚"),
			ini: &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{},
				cfg:        &initConfig{},
			},
			execution: func(i *initializer, cfg *initConfig) error {
				regio, err := askRegion(i.stdio)
				cfg.region = regio
				return err
			},
			expectedState: &initConfig{region: region.EUCentral1.String()},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func TestAskDownloadWhen(t *testing.T) {
	testCases := []questionTest{
		{
			name:      "Defaults to Fail",
			procedure: stringToProcedure("✓🔚"),
			ini: &initializer{
				cfg: &initConfig{},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askDownloadWhen()
			},
			expectedState: &initConfig{artifactWhen: config.WhenFail},
		},
		{
			name:      "Second is pass",
			procedure: stringToProcedure("↓✓🔚"),
			ini: &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{},
				cfg:        &initConfig{},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askDownloadWhen()
			},
			expectedState: &initConfig{artifactWhen: config.WhenPass},
		},
		{
			name:      "Type always",
			procedure: stringToProcedure("alw✓🔚"),
			ini: &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{},
				cfg:        &initConfig{},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askDownloadWhen()
			},
			expectedState: &initConfig{artifactWhen: config.WhenAlways},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func TestAskDevice(t *testing.T) {
	devs := []string{"Google Pixel 3", "Google Pixel 4"}
	testCases := []questionTest{
		{
			name:      "Default Device",
			procedure: stringToProcedure("✓🔚"),
			ini: &initializer{
				cfg: &initConfig{},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askDevice(devs)
			},
			expectedState: &initConfig{device: config.Device{Name: "Google Pixel 3"}},
		},
		{
			name:      "Input is captured",
			procedure: stringToProcedure("Pixel 4✓🔚"),
			ini: &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{},
				cfg:        &initConfig{},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askDevice(devs)
			},
			expectedState: &initConfig{device: config.Device{Name: "Google Pixel 4"}},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func TestAskEmulator(t *testing.T) {
	vmds := []vmd.VirtualDevice{
		{Name: "Google Pixel 3 Emulator", OSVersion: []string{"9.0", "8.0", "7.0"}},
		{Name: "Google Pixel 4 Emulator", OSVersion: []string{"9.0", "8.0", "7.0"}},
	}
	testCases := []questionTest{
		{
			name: "Empty is allowed",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select emulator:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform version:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				cfg: &initConfig{},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askEmulator(vmds)
			},
			expectedState: &initConfig{emulator: config.Emulator{Name: "Google Pixel 3 Emulator", PlatformVersions: []string{"9.0"}}},
		},
		{
			name: "Input is captured",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select emulator:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Pixel 4")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform version:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("7.0")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{},
				cfg:        &initConfig{},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askEmulator(vmds)
			},
			expectedState: &initConfig{emulator: config.Emulator{Name: "Google Pixel 4 Emulator", PlatformVersions: []string{"7.0"}}},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func TestAskPlatform(t *testing.T) {
	metas := []framework.Metadata{
		{
			FrameworkName:    testcafe.Kind,
			FrameworkVersion: "1.5.0",
			DockerImage:      "dummy-docker-image",
			Platforms: []framework.Platform{
				{
					PlatformName: "Windows 10",
					BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
				},
				{
					PlatformName: "macOS 11.00",
					BrowserNames: []string{"safari", "googlechrome", "firefox", "microsoftedge"},
				},
			},
		},
		{
			FrameworkName:    testcafe.Kind,
			FrameworkVersion: "1.3.0",
			Platforms: []framework.Platform{
				{
					PlatformName: "Windows 10",
					BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
				},
				{
					PlatformName: "macOS 11.00",
					BrowserNames: []string{"safari", "googlechrome", "firefox", "microsoftedge"},
				},
			},
		},
	}

	testCases := []questionTest{
		{
			name: "Windows 10",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select browser")
				if err != nil {
					return err
				}
				_, err = c.SendLine("chrome")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Windows 10")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				cfg: &initConfig{
					frameworkName:    testcafe.Kind,
					frameworkVersion: "1.5.0",
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askPlatform(metas)
			},
			expectedState: &initConfig{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.5.0",
				browserName:      "chrome",
				platformName:     "Windows 10",
			},
		},
		{
			name: "macOS",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select browser")
				if err != nil {
					return err
				}
				_, err = c.SendLine("firefox")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform")
				if err != nil {
					return err
				}
				_, err = c.SendLine("macOS")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				cfg: &initConfig{
					frameworkName:    testcafe.Kind,
					frameworkVersion: "1.5.0",
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askPlatform(metas)
			},
			expectedState: &initConfig{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.5.0",
				platformName:     "macOS 11.00",
				browserName:      "firefox",
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func TestAskVersion(t *testing.T) {
	metas := []framework.Metadata{
		{
			FrameworkName:    testcafe.Kind,
			FrameworkVersion: "1.5.0",
			Platforms: []framework.Platform{
				{
					PlatformName: "macOS 11.00",
					BrowserNames: []string{"safari", "googlechrome", "firefox", "microsoftedge"},
				},
				{
					PlatformName: "Windows 10",
					BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
				},
			},
		},
		{
			FrameworkName:    testcafe.Kind,
			FrameworkVersion: "1.3.0",
			Platforms: []framework.Platform{
				{
					PlatformName: "macOS 11.00",
					BrowserNames: []string{"safari", "googlechrome", "firefox", "microsoftedge"},
				},
				{
					PlatformName: "Windows 10",
					BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
				},
			},
		},
	}

	testCases := []questionTest{
		{
			name: "Default",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select testcafe version")
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
			ini: &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{},
				cfg: &initConfig{
					frameworkName: testcafe.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askVersion(metas)
			},
			expectedState: &initConfig{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.5.0",
			},
		},
		{
			name: "Second",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select testcafe version")
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
			ini: &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{},
				cfg: &initConfig{
					frameworkName: testcafe.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.askVersion(metas)
			},
			expectedState: &initConfig{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.3.0",
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
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
			procedure: func(c *expect.Console) error {
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
			ini: &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{},
				cfg:        &initConfig{},
			},
			execution: func(i *initializer, cfg *initConfig) error {
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
			expectedState: &initConfig{
				app: dir.Join("android-app.apk"),
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func TestConfigure(t *testing.T) {
	dir := fs.NewDir(t, "apps",
		fs.WithFile("cypress.json", "{}", fs.WithMode(0644)),
		fs.WithFile("android-app.apk", "myAppContent", fs.WithMode(0644)),
		fs.WithFile("ios-app.ipa", "myAppContent", fs.WithMode(0644)),
		fs.WithDir("ios-folder-app.app", fs.WithMode(0755)))
	defer dir.Remove()

	frameworkVersions := []framework.Metadata{
		{
			FrameworkName:    cypress.Kind,
			FrameworkVersion: "7.5.0",
			DockerImage:      "dummy-docker-image",
			Platforms: []framework.Platform{
				{
					PlatformName: "windows 10",
					BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
				},
			},
		},
	}
	ir := &mocks.FakeFrameworkInfoReader{
		VersionsFn: func(context.Context, string) ([]framework.Metadata, error) {
			return frameworkVersions, nil
		},
		FrameworksFn: func(context.Context) ([]string, error) {
			return []string{cypress.Kind, espresso.Kind}, nil
		},
	}
	dr := &mocks.FakeDevicesReader{
		GetDevicesFn: func(context.Context, string) ([]devices.Device, error) {
			return []devices.Device{
				{Name: "Google Pixel 3"},
				{Name: "Google Pixel 4"},
			}, nil
		},
	}
	er := &mocks.FakeEmulatorsReader{
		GetVirtualDevicesFn: func(context.Context, string) ([]vmd.VirtualDevice, error) {
			return []vmd.VirtualDevice{
				{Name: "Google Pixel Emulator", OSVersion: []string{"9.0", "8.0", "7.0"}},
				{Name: "Samsung Galaxy Emulator", OSVersion: []string{"9.0", "8.0", "7.0"}},
			}, nil
		},
	}

	testCases := []questionTest{
		{
			name: "Complete Configuration (espresso)",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Application to test")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("android-app.apk"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Test application")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("android-app.apk"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select device pattern:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Google Pixel .*")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select emulator:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Google Pixel Emulator")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform version:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("7.0")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Download artifacts:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("when tests are passing")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader:   ir,
				deviceReader: dr,
				vmdReader:    er,
				cfg: &initConfig{
					frameworkName: espresso.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.configure()
			},
			expectedState: &initConfig{
				frameworkName: espresso.Kind,
				app:           dir.Join("android-app.apk"),
				testApp:       dir.Join("android-app.apk"),
				emulator:      config.Emulator{Name: "Google Pixel Emulator", PlatformVersions: []string{"7.0"}},
				device:        config.Device{Name: "Google Pixel .*"},
				artifactWhen:  config.WhenPass,
			},
		},
		{
			name: "Complete Configuration (cypress)",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select cypress version")
				if err != nil {
					return err
				}
				_, err = c.SendLine("7.5.0")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Cypress configuration file:")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("cypress.json"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select browser:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("chrome")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Windows 10")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Download artifacts:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("when tests are passing")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader:   ir,
				deviceReader: dr,
				vmdReader:    er,
				cfg: &initConfig{
					frameworkName: cypress.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.configure()
			},
			expectedState: &initConfig{
				frameworkName:     cypress.Kind,
				frameworkVersion:  "7.5.0",
				cypressConfigFile: dir.Join("cypress.json"),
				platformName:      "Windows 10",
				browserName:       "chrome",
				artifactWhen:      config.WhenPass,
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func TestAskCredentials(t *testing.T) {
	testCases := []questionTest{
		{
			name: "Default",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Sauce Labs username:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("dummy-user")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Sauce Labs access key:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("dummy-access-key")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				cfg: &initConfig{},
			},
			execution: func(i *initializer, _ *initConfig) error {
				creds, err := askCredentials(i.stdio)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				expect := &iam.Credentials{Username: "dummy-user", AccessKey: "dummy-access-key"}
				if reflect.DeepEqual(creds, expect) {
					t.Fatalf("got: %v, want: %v", creds, expect)
				}
				return nil
			},
			expectedState: &initConfig{},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func Test_initializers(t *testing.T) {
	dir := fs.NewDir(t, "apps",
		fs.WithFile("cypress.json", "{}", fs.WithMode(0644)),
		fs.WithFile("android-app.apk", "myAppContent", fs.WithMode(0644)),
		fs.WithFile("ios-app.ipa", "myAppContent", fs.WithMode(0644)),
		fs.WithDir("ios-folder-app.app", fs.WithMode(0755)))
	defer dir.Remove()

	frameworkVersions := map[string][]framework.Metadata{
		cypress.Kind: {
			{
				FrameworkName:    cypress.Kind,
				FrameworkVersion: "7.5.0",
				DockerImage:      "dummy-docker-image",
				Platforms: []framework.Platform{
					{
						PlatformName: "windows 10",
						BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
					},
				},
			},
		},
		playwright.Kind: {
			{
				FrameworkName:    playwright.Kind,
				FrameworkVersion: "1.11.0",
				DockerImage:      "dummy-docker-image",
				Platforms: []framework.Platform{
					{
						PlatformName: "windows 10",
						BrowserNames: []string{"playwright-chromium", "playwright-firefox", "playwright-webkit"},
					},
				},
			},
		},
		testcafe.Kind: {
			{
				FrameworkName:    testcafe.Kind,
				FrameworkVersion: "1.12.0",
				DockerImage:      "dummy-docker-image",
				Platforms: []framework.Platform{
					{
						PlatformName: "windows 10",
						BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
					},
					{
						PlatformName: "macOS 11.00",
						BrowserNames: []string{"googlechrome", "firefox", "microsoftedge", "safari"},
					},
				},
			},
		},
		imagerunner.Kind: {
			{
				FrameworkName:    imagerunner.Kind,
				FrameworkVersion: "1.0.0",
			},
		},
	}
	ir := &mocks.FakeFrameworkInfoReader{
		VersionsFn: func(_ context.Context, frameworkName string) ([]framework.Metadata, error) {
			return frameworkVersions[frameworkName], nil
		},
		FrameworksFn: func(context.Context) ([]string, error) {
			return []string{
				cypress.Kind,
				espresso.Kind,
				imagerunner.Kind,
				playwright.Kind,
				testcafe.Kind,
				xcuitest.Kind,
			}, nil
		},
	}

	er := &mocks.FakeEmulatorsReader{
		GetVirtualDevicesFn: func(context.Context, string) ([]vmd.VirtualDevice, error) {
			return []vmd.VirtualDevice{
				{Name: "Google Pixel Emulator", OSVersion: []string{"9.0", "8.0", "7.0"}},
				{Name: "Samsung Galaxy Emulator", OSVersion: []string{"9.0", "8.0", "7.0"}},
			}, nil
		},
	}

	testCases := []questionTest{
		{
			name: "Cypress - Windows 10 - chrome",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select cypress version")
				if err != nil {
					return err
				}
				_, err = c.SendLine("7.5.0")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Cypress configuration file:")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("cypress.json"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select browser:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("chrome")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Windows 10")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Download artifacts:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("when tests are passing")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader: ir,
				cfg: &initConfig{
					frameworkName: cypress.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.initializeCypress()
			},
			expectedState: &initConfig{
				frameworkName:     cypress.Kind,
				frameworkVersion:  "7.5.0",
				cypressConfigFile: dir.Join("cypress.json"),
				platformName:      "Windows 10",
				browserName:       "chrome",
				artifactWhen:      config.WhenPass,
			},
		},
		{
			name: "Playwright - Windows 10 - chromium",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select playwright version")
				if err != nil {
					return err
				}
				_, err = c.SendLine("1.11.0")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select browser:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("chromium")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Windows 10")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Playwright project name. Leave empty if your configuration does not contain projects:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Test file pattern to match against:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Download artifacts:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("when tests are passing")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader: ir,
				cfg: &initConfig{
					frameworkName: playwright.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.initializePlaywright()
			},
			expectedState: &initConfig{
				frameworkName:    playwright.Kind,
				frameworkVersion: "1.11.0",
				platformName:     "Windows 10",
				browserName:      "chromium",
				artifactWhen:     config.WhenPass,
				testMatch:        []string{".*.spec.js"},
			},
		},
		{
			name: "Testcafe - macOS 11.00 - safari",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select testcafe version")
				if err != nil {
					return err
				}
				_, err = c.SendLine("1.12.0")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select browser:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("safari")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("macOS 11.00")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Download artifacts:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("when tests are passing")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader: ir,
				cfg: &initConfig{
					frameworkName: testcafe.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.initializeTestcafe()
			},
			expectedState: &initConfig{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.12.0",
				platformName:     "macOS 11.00",
				browserName:      "safari",
				artifactWhen:     config.WhenPass,
			},
		},
		{
			name: "XCUITest - .ipa",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select target:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Real Devices")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select device pattern:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("iPhone .*")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Application to test:")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("ios-app.ipa"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Test application:")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("ios-app.ipa"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Download artifacts:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("when tests are passing")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader: ir,
				cfg: &initConfig{
					frameworkName: xcuitest.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.initializeXCUITest()
			},
			expectedState: &initConfig{
				frameworkName: xcuitest.Kind,
				app:           dir.Join("ios-app.ipa"),
				testApp:       dir.Join("ios-app.ipa"),
				device:        config.Device{Name: "iPhone .*"},
				artifactWhen:  config.WhenPass,
			},
		},
		{
			name: "XCUITest - .app",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Select target:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Real Devices")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select device pattern:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("iPad .*")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Application to test:")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("ios-folder-app.app"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Test application:")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("ios-folder-app.app"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Download artifacts:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("when tests are passing")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader: ir,
				cfg: &initConfig{
					frameworkName: xcuitest.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.initializeXCUITest()
			},
			expectedState: &initConfig{
				frameworkName: xcuitest.Kind,
				app:           dir.Join("ios-folder-app.app"),
				testApp:       dir.Join("ios-folder-app.app"),
				device:        config.Device{Name: "iPad .*"},
				artifactWhen:  config.WhenPass,
			},
		},
		{
			name: "Espresso - .apk",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Application to test:")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("android-app.apk"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Test application:")
				if err != nil {
					return err
				}
				_, err = c.SendLine(dir.Join("android-app.apk"))
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select device pattern:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("HTC .*")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select emulator:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Samsung Galaxy Emulator")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Select platform version:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("8.0")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Download artifacts:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("when tests are passing")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader: ir,
				vmdReader:  er,
				cfg: &initConfig{
					frameworkName: espresso.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.initializeEspresso()
			},
			expectedState: &initConfig{
				frameworkName: espresso.Kind,
				app:           dir.Join("android-app.apk"),
				testApp:       dir.Join("android-app.apk"),
				device:        config.Device{Name: "HTC .*"},
				emulator:      config.Emulator{Name: "Samsung Galaxy Emulator", PlatformVersions: []string{"8.0"}},
				artifactWhen:  config.WhenPass,
			},
		},
		{
			name: "ImageRunner - DockerImage",
			procedure: func(c *expect.Console) error {
				_, err := c.ExpectString("Docker Image to use:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("ubuntu:latest")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Set workload:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("webdriver")
				if err != nil {
					return err
				}
				_, err = c.ExpectString("Download artifacts:")
				if err != nil {
					return err
				}
				_, err = c.SendLine("when tests are passing")
				if err != nil {
					return err
				}
				_, err = c.ExpectEOF()
				if err != nil {
					return err
				}
				return nil
			},
			ini: &initializer{
				infoReader: ir,
				cfg: &initConfig{
					frameworkName: imagerunner.Kind,
				},
			},
			execution: func(i *initializer, _ *initConfig) error {
				return i.initializeImageRunner()
			},
			expectedState: &initConfig{
				frameworkName: imagerunner.Kind,
				dockerImage:   "ubuntu:latest",
				workload:      "webdriver",
				artifactWhen:  config.WhenPass,
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func Test_metaToBrowsers(t *testing.T) {
	type args struct {
		metadatas        []framework.Metadata
		frameworkName    string
		frameworkVersion string
	}
	tests := []struct {
		name          string
		args          args
		wantBrowsers  []string
		wantPlatforms map[string][]string
	}{
		{
			name: "1 version / 1 platform",
			args: args{
				frameworkName:    "framework",
				frameworkVersion: "1.1.0",
				metadatas: []framework.Metadata{
					{
						FrameworkName:    "framework",
						FrameworkVersion: "1.1.0",
						Platforms: []framework.Platform{
							{
								PlatformName: "windows 10",
								BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
							},
						},
					},
				},
			},
			wantBrowsers: []string{"chrome", "firefox", "microsoftedge"},
			wantPlatforms: map[string][]string{
				"chrome":        {"Windows 10"},
				"firefox":       {"Windows 10"},
				"microsoftedge": {"Windows 10"},
			},
		},
		{
			name: "1 version / docker only",
			args: args{
				frameworkName:    "framework",
				frameworkVersion: "1.1.0",
				metadatas: []framework.Metadata{
					{
						FrameworkName:    "framework",
						DockerImage:      "framework-images",
						FrameworkVersion: "1.1.0",
						Platforms:        []framework.Platform{},
					},
				},
			},
			wantPlatforms: map[string][]string{},
		},
		{
			name: "1 version / 1 platform + docker",
			args: args{
				frameworkName:    "framework",
				frameworkVersion: "1.1.0",
				metadatas: []framework.Metadata{
					{
						FrameworkName:    "framework",
						DockerImage:      "framework-image:latest",
						FrameworkVersion: "1.1.0",
						Platforms: []framework.Platform{
							{
								PlatformName: "Windows 10",
								BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
							},
						},
					},
				},
			},
			wantBrowsers: []string{"chrome", "firefox", "microsoftedge"},
			wantPlatforms: map[string][]string{
				"chrome":        {"Windows 10"},
				"firefox":       {"Windows 10"},
				"microsoftedge": {"Windows 10"},
			},
		},
		{
			name: "1 version / 2 platform + docker",
			args: args{
				frameworkName:    "framework",
				frameworkVersion: "1.1.0",
				metadatas: []framework.Metadata{
					{
						FrameworkName:    "framework",
						DockerImage:      "framework-image:latest",
						FrameworkVersion: "1.1.0",
						Platforms: []framework.Platform{
							{
								PlatformName: "Windows 10",
								BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
							},
							{
								PlatformName: "macOS 11.00",
								BrowserNames: []string{"googlechrome", "firefox", "safari", "microsoftedge"},
							},
						},
					},
				},
			},
			wantBrowsers: []string{"chrome", "firefox", "microsoftedge", "safari"},
			wantPlatforms: map[string][]string{
				"chrome":        {"Windows 10", "macOS 11.00"},
				"firefox":       {"Windows 10", "macOS 11.00"},
				"microsoftedge": {"Windows 10", "macOS 11.00"},
				"safari":        {"macOS 11.00"},
			},
		},
		{
			name: "2 version / 2 platform + docker",
			args: args{
				frameworkName:    "framework",
				frameworkVersion: "1.1.0",
				metadatas: []framework.Metadata{
					{
						FrameworkName:    "framework",
						DockerImage:      "framework-image:latest",
						FrameworkVersion: "1.2.0",
						Platforms: []framework.Platform{
							{
								PlatformName: "Windows 10",
								BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
							},
						},
					},
					{
						FrameworkName:    "framework",
						DockerImage:      "framework-image:latest",
						FrameworkVersion: "1.1.0",
						Platforms: []framework.Platform{
							{
								PlatformName: "Windows 10",
								BrowserNames: []string{"googlechrome", "firefox", "microsoftedge"},
							},
							{
								PlatformName: "macOS 11.00",
								BrowserNames: []string{"googlechrome", "firefox", "safari", "microsoftedge"},
							},
						},
					},
				},
			},
			wantBrowsers: []string{"chrome", "firefox", "microsoftedge", "safari"},
			wantPlatforms: map[string][]string{
				"chrome":        {"Windows 10", "macOS 11.00"},
				"firefox":       {"Windows 10", "macOS 11.00"},
				"microsoftedge": {"Windows 10", "macOS 11.00"},
				"safari":        {"macOS 11.00"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBrowsers, gotPlatforms := metaToBrowsers(tt.args.metadatas, tt.args.frameworkName, tt.args.frameworkVersion)
			if !reflect.DeepEqual(gotBrowsers, tt.wantBrowsers) {
				t.Errorf("metaToBrowsers() got = %v, want %v", gotBrowsers, tt.wantBrowsers)
			}
			if !reflect.DeepEqual(gotPlatforms, tt.wantPlatforms) {
				t.Errorf("metaToBrowsers() got = %v, want %v", gotPlatforms, tt.wantPlatforms)
			}
		})
	}
}

func Test_checkCredentials(t *testing.T) {
	tests := []struct {
		name        string
		frameworkFn func(ctx context.Context) ([]string, error)
		wantErr     error
	}{
		{
			name: "Success",
			frameworkFn: func(context.Context) ([]string, error) {
				return []string{cypress.Kind}, nil
			},
			wantErr: nil,
		},
		{
			name: "Invalid credentials",
			frameworkFn: func(context.Context) ([]string, error) {
				errMsg := "unexpected status '401' from test-composer: Unauthorized\n"
				return []string{}, errors.New(errMsg)
			},
			wantErr: errors.New("invalid credentials provided"),
		},
		{
			name: "Other error",
			frameworkFn: func(context.Context) ([]string, error) {
				return []string{}, errors.New("other error")
			},
			wantErr: errors.New("other error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ini := &initializer{
				infoReader: &mocks.FakeFrameworkInfoReader{
					FrameworksFn: tt.frameworkFn,
				},
			}
			if err := ini.checkCredentials("us-west-1"); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("checkCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_checkFrameworkVersion(t *testing.T) {
	metadatas := []framework.Metadata{
		{
			FrameworkName:    testcafe.Kind,
			FrameworkVersion: "1.0.0",
			Platforms: []framework.Platform{
				{
					PlatformName: "windows 10",
					BrowserNames: []string{"chrome", "firefox"},
				},
				{
					PlatformName: "macos 11.00",
					BrowserNames: []string{"chrome", "firefox", "safari"},
				},
				{
					PlatformName: "docker",
					BrowserNames: []string{"chrome", "firefox"},
				},
			},
		},
	}
	type args struct {
		frameworkName    string
		frameworkVersion string
		metadatas        []framework.Metadata
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "Available version",
			args: args{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.0.0",
				metadatas:        metadatas,
			},
			wantErr: nil,
		},
		{
			name: "Unavailable version",
			args: args{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "buggy-version",
				metadatas:        metadatas,
			},
			wantErr: errors.New("testcafe buggy-version is not supported. Supported versions are: 1.0.0"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkFrameworkVersion(tt.args.metadatas, tt.args.frameworkName, tt.args.frameworkVersion)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("checkFrameworkVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_checkBrowserAndPlatform(t *testing.T) {
	metadatas := []framework.Metadata{
		{
			FrameworkName:    testcafe.Kind,
			FrameworkVersion: "1.0.0",
			Platforms: []framework.Platform{
				{
					PlatformName: "windows 10",
					BrowserNames: []string{"chrome", "firefox"},
				},
				{
					PlatformName: "macos 11.00",
					BrowserNames: []string{"chrome", "firefox", "safari"},
				},
				{
					PlatformName: "docker",
					BrowserNames: []string{"chrome", "firefox"},
				},
			},
		},
	}

	type args struct {
		frameworkName    string
		frameworkVersion string
		browserName      string
		platformName     string
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "Default",
			args: args{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.0.0",
				platformName:     "Windows 10",
				browserName:      "chrome",
			},
			wantErr: nil,
		},
		{
			name: "Unavailable browser",
			args: args{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.0.0",
				platformName:     "Windows 10",
				browserName:      "webkit",
			},
			wantErr: errors.New("webkit: unsupported browser. Supported browsers are: chrome, firefox, safari"),
		},
		{
			name: "Unavailable browser on platform",
			args: args{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.0.0",
				platformName:     "Windows 10",
				browserName:      "safari",
			},
			wantErr: errors.New("safari: unsupported browser on Windows 10"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkBrowserAndPlatform(metadatas, tt.args.frameworkName, tt.args.frameworkVersion, tt.args.browserName, tt.args.platformName)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("checkBrowserAndPlatform() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_checkArtifactDownloadSetting(t *testing.T) {
	type args struct {
		when string
	}
	tests := []struct {
		name    string
		args    args
		want    config.When
		wantErr bool
	}{
		{
			name: `Passing: fail`,
			args: args{
				when: "fail",
			},
			want:    config.WhenFail,
			wantErr: false,
		},
		{
			name: `Invalid kind`,
			args: args{
				when: "dummy-value",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkArtifactDownloadSetting(tt.args.when)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkArtifactDownloadSetting() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("checkArtifactDownloadSetting() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkEmulators(t *testing.T) {
	vmds := []vmd.VirtualDevice{
		{
			Name: "Google Api Emulator",
			OSVersion: []string{
				"11.0",
				"10.0",
				"9.0",
			},
		},
		{
			Name: "Samsung Galaxy Emulator",
			OSVersion: []string{
				"11.0",
				"10.0",
				"8.1",
				"8.0",
			},
		},
	}

	type args struct {
		emulator config.Emulator
	}
	tests := []struct {
		name     string
		args     args
		want     config.Emulator
		wantErrs []error
	}{
		{
			name: "single version",
			args: args{
				emulator: config.Emulator{
					Name:             "Google Api Emulator",
					PlatformVersions: []string{"10.0"},
				},
			},
			want: config.Emulator{
				Name:             "Google Api Emulator",
				PlatformVersions: []string{"10.0"},
			},
			wantErrs: []error{},
		},
		{
			name: "multiple versions",
			args: args{
				emulator: config.Emulator{
					Name:             "Google Api Emulator",
					PlatformVersions: []string{"10.0", "9.0"},
				},
			},
			want: config.Emulator{
				Name:             "Google Api Emulator",
				PlatformVersions: []string{"10.0", "9.0"},
			},
			wantErrs: []error{},
		},
		{
			name: "multiple + buggy versions",
			args: args{
				emulator: config.Emulator{
					Name:             "Google Api Emulator",
					PlatformVersions: []string{"10.0", "8.1"},
				},
			},
			want:     config.Emulator{},
			wantErrs: []error{errors.New("emulator: Google Api Emulator does not support platform 8.1")},
		},
		{
			name: "case sensitiveness correction",
			args: args{
				emulator: config.Emulator{
					Name:             "google api emulator",
					PlatformVersions: []string{"10.0"},
				},
			},
			want: config.Emulator{
				Name:             "Google Api Emulator",
				PlatformVersions: []string{"10.0"},
			},
			wantErrs: []error{},
		},
		{
			name: "invalid emulator",
			args: args{
				emulator: config.Emulator{
					Name:             "buggy emulator",
					PlatformVersions: []string{"10.0"},
				},
			},
			want:     config.Emulator{},
			wantErrs: []error{errors.New("emulator: buggy emulator does not exists")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errs := checkEmulators(vmds, tt.args.emulator)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("checkEmulators() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(errs, tt.wantErrs) {
				t.Errorf("checkEmulators() got = %v, want %v", errs, tt.wantErrs)
			}
		})
	}
}

func Test_initializer_initializeBatchCypress(t *testing.T) {
	dir := fs.NewDir(t, "apps",
		fs.WithFile("cypress.json", "{}", fs.WithMode(0644)))
	defer dir.Remove()

	ini := &initializer{
		infoReader: &mocks.FakeFrameworkInfoReader{VersionsFn: func(context.Context, string) ([]framework.Metadata, error) {
			return []framework.Metadata{
				{
					FrameworkName:    cypress.Kind,
					FrameworkVersion: "7.0.0",
					Platforms: []framework.Platform{
						{
							PlatformName: "windows 10",
							BrowserNames: []string{"chrome", "firefox"},
						},
					},
				},
			}, nil
		}},
		userService: &mocks.UserService{ConcurrencyFn: func(context.Context) (iam.Concurrency, error) {
			return iam.Concurrency{
				Org: iam.OrgConcurrency{
					Allowed: iam.CloudConcurrency{
						VDC: 2,
					},
				},
			}, nil
		}},
		cfg: &initConfig{},
	}
	var emptyErr []error

	type args struct {
		initCfg *initConfig
	}
	tests := []struct {
		name     string
		args     args
		want     *initConfig
		wantErrs []error
	}{
		{
			name: "Basic",
			args: args{
				initCfg: &initConfig{
					frameworkName:     cypress.Kind,
					frameworkVersion:  "7.0.0",
					browserName:       "chrome",
					platformName:      "Windows 10",
					cypressConfigFile: dir.Join("cypress.json"),
					region:            "us-west-1",
					artifactWhen:      "fail",
				},
			},
			want: &initConfig{
				frameworkName:     cypress.Kind,
				frameworkVersion:  "7.0.0",
				browserName:       "chrome",
				platformName:      "Windows 10",
				cypressConfigFile: dir.Join("cypress.json"),
				region:            "us-west-1",
				artifactWhen:      config.WhenFail,
			},
			wantErrs: emptyErr,
		},
		{
			name: "invalid browser/platform",
			args: args{
				initCfg: &initConfig{
					frameworkName:    cypress.Kind,
					frameworkVersion: "7.0.0",
					browserName:      "dummy",
					platformName:     "dummy",
					artifactWhenStr:  "dummy",
				},
			},
			want: &initConfig{
				frameworkName:    cypress.Kind,
				frameworkVersion: "7.0.0",
				browserName:      "dummy",
				platformName:     "dummy",
				artifactWhenStr:  "dummy",
			},
			wantErrs: []error{
				errors.New("no cypress config file specified"),
				errors.New("dummy: unsupported browser. Supported browsers are: chrome, firefox"),
				errors.New("dummy: unknown download condition"),
			},
		},
		{
			name: "no flags",
			args: args{
				initCfg: &initConfig{
					frameworkName: cypress.Kind,
				},
			},
			want: &initConfig{
				frameworkName: cypress.Kind,
			},
			wantErrs: []error{
				errors.New("no cypress version specified"),
				errors.New("no cypress config file specified"),
				errors.New("no platform name specified"),
				errors.New("no browser name specified"),
			},
		},
		{
			name: "invalid framework version / Invalid config file",
			args: args{
				initCfg: &initConfig{
					frameworkName:     cypress.Kind,
					frameworkVersion:  "8.0.0",
					cypressConfigFile: "/my/fake/cypress.json",
				},
			},
			want: &initConfig{
				frameworkName:     cypress.Kind,
				frameworkVersion:  "8.0.0",
				cypressConfigFile: "/my/fake/cypress.json",
			},
			wantErrs: []error{
				errors.New("no platform name specified"),
				errors.New("no browser name specified"),
				errors.New("cypress 8.0.0 is not supported. Supported versions are: 7.0.0"),
				errors.New("/my/fake/cypress.json: stat /my/fake/cypress.json: no such file or directory"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ini.cfg = tt.args.initCfg
			errs := ini.initializeBatchCypress()
			if !reflect.DeepEqual(ini.cfg, tt.want) {
				t.Errorf("initializeBatchCypress() got = %v, want %v", ini.cfg, tt.want)
			}
			if !reflect.DeepEqual(errs, tt.wantErrs) {
				t.Errorf("initializeBatchCypress() got = %v, want %v", errs, tt.wantErrs)
			}
		})
	}
}

func Test_initializer_initializeBatchTestcafe(t *testing.T) {
	ini := &initializer{
		infoReader: &mocks.FakeFrameworkInfoReader{VersionsFn: func(context.Context, string) ([]framework.Metadata, error) {
			return []framework.Metadata{
				{
					FrameworkName:    testcafe.Kind,
					FrameworkVersion: "1.0.0",
					Platforms: []framework.Platform{
						{
							PlatformName: "windows 10",
							BrowserNames: []string{"chrome", "firefox"},
						},
						{
							PlatformName: "macOS 11.00",
							BrowserNames: []string{"chrome", "firefox", "safari"},
						},
					},
				},
			}, nil
		}},
		userService: &mocks.UserService{ConcurrencyFn: func(context.Context) (iam.Concurrency, error) {
			return iam.Concurrency{
				Org: iam.OrgConcurrency{
					Allowed: iam.CloudConcurrency{
						VDC: 2,
					},
				},
			}, nil
		}},
		cfg: &initConfig{},
	}
	var emptyErr []error

	type args struct {
		initCfg *initConfig
	}
	tests := []struct {
		name     string
		args     args
		want     *initConfig
		wantErrs []error
	}{
		{
			name: "Basic",
			args: args{
				initCfg: &initConfig{
					frameworkName:    testcafe.Kind,
					frameworkVersion: "1.0.0",
					browserName:      "chrome",
					platformName:     "Windows 10",
					region:           "us-west-1",
					artifactWhen:     "fail",
				},
			},
			want: &initConfig{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.0.0",
				browserName:      "chrome",
				platformName:     "Windows 10",
				region:           "us-west-1",
				artifactWhen:     config.WhenFail,
			},
			wantErrs: emptyErr,
		},
		{
			name: "invalid browser/platform",
			args: args{
				initCfg: &initConfig{
					frameworkName:    testcafe.Kind,
					frameworkVersion: "1.0.0",
					browserName:      "dummy",
					platformName:     "dummy",
					artifactWhenStr:  "dummy",
				},
			},
			want: &initConfig{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "1.0.0",
				browserName:      "dummy",
				platformName:     "dummy",
				artifactWhenStr:  "dummy",
			},
			wantErrs: []error{
				errors.New("dummy: unsupported browser. Supported browsers are: chrome, firefox, safari"),
				errors.New("dummy: unknown download condition"),
			},
		},
		{
			name: "no flags",
			args: args{
				initCfg: &initConfig{
					frameworkName: testcafe.Kind,
				},
			},
			want: &initConfig{
				frameworkName: testcafe.Kind,
			},
			wantErrs: []error{
				errors.New("no testcafe version specified"),
				errors.New("no platform name specified"),
				errors.New("no browser name specified"),
			},
		},
		{
			name: "invalid framework version / Invalid config file",
			args: args{
				initCfg: &initConfig{
					frameworkName:    testcafe.Kind,
					frameworkVersion: "8.0.0",
				},
			},
			want: &initConfig{
				frameworkName:    testcafe.Kind,
				frameworkVersion: "8.0.0",
			},
			wantErrs: []error{
				errors.New("no platform name specified"),
				errors.New("no browser name specified"),
				errors.New("testcafe 8.0.0 is not supported. Supported versions are: 1.0.0"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ini.cfg = tt.args.initCfg
			errs := ini.initializeBatchTestcafe()
			if !reflect.DeepEqual(ini.cfg, tt.want) {
				t.Errorf("initializeBatchTestcafe() got = %v, want %v", ini.cfg, tt.want)
			}
			if !reflect.DeepEqual(errs, tt.wantErrs) {
				t.Errorf("initializeBatchTestcafe() got = %v, want %v", errs, tt.wantErrs)
			}
		})
	}
}

func Test_initializer_initializeBatchPlaywright(t *testing.T) {
	ini := &initializer{
		infoReader: &mocks.FakeFrameworkInfoReader{VersionsFn: func(context.Context, string) ([]framework.Metadata, error) {
			return []framework.Metadata{
				{
					FrameworkName:    playwright.Kind,
					FrameworkVersion: "1.0.0",
					Platforms: []framework.Platform{
						{
							PlatformName: "windows 10",
							BrowserNames: []string{"chromium", "firefox", "webkit"},
						},
					},
				},
			}, nil
		}},
		userService: &mocks.UserService{ConcurrencyFn: func(context.Context) (iam.Concurrency, error) {
			return iam.Concurrency{
				Org: iam.OrgConcurrency{
					Allowed: iam.CloudConcurrency{
						VDC: 2,
					},
				},
			}, nil
		}},
		cfg: &initConfig{},
	}
	var emptyErr []error

	type args struct {
		initCfg *initConfig
	}
	tests := []struct {
		name     string
		args     args
		want     *initConfig
		wantErrs []error
	}{
		{
			name: "Basic",
			args: args{
				initCfg: &initConfig{
					frameworkName:    playwright.Kind,
					frameworkVersion: "1.0.0",
					browserName:      "chromium",
					platformName:     "Windows 10",
					region:           "us-west-1",
					artifactWhen:     "fail",
				},
			},
			want: &initConfig{
				frameworkName:    playwright.Kind,
				frameworkVersion: "1.0.0",
				browserName:      "chromium",
				platformName:     "Windows 10",
				region:           "us-west-1",
				artifactWhen:     config.WhenFail,
			},
			wantErrs: emptyErr,
		},
		{
			name: "invalid browser/platform",
			args: args{
				initCfg: &initConfig{
					frameworkName:    playwright.Kind,
					frameworkVersion: "1.0.0",
					browserName:      "dummy",
					platformName:     "dummy",
					artifactWhenStr:  "dummy",
				},
			},
			want: &initConfig{
				frameworkName:    playwright.Kind,
				frameworkVersion: "1.0.0",
				browserName:      "dummy",
				platformName:     "dummy",
				artifactWhenStr:  "dummy",
			},
			wantErrs: []error{
				errors.New("dummy: unsupported browser. Supported browsers are: chromium, firefox, webkit"),
				errors.New("dummy: unknown download condition"),
			},
		},
		{
			name: "no flags",
			args: args{
				initCfg: &initConfig{
					frameworkName: playwright.Kind,
				},
			},
			want: &initConfig{
				frameworkName: playwright.Kind,
			},
			wantErrs: []error{
				errors.New("no playwright version specified"),
				errors.New("no platform name specified"),
				errors.New("no browser name specified"),
			},
		},
		{
			name: "invalid framework version / Invalid config file",
			args: args{
				initCfg: &initConfig{
					frameworkName:    playwright.Kind,
					frameworkVersion: "8.0.0",
				},
			},
			want: &initConfig{
				frameworkName:    playwright.Kind,
				frameworkVersion: "8.0.0",
			},
			wantErrs: []error{
				errors.New("no platform name specified"),
				errors.New("no browser name specified"),
				errors.New("playwright 8.0.0 is not supported. Supported versions are: 1.0.0"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ini.cfg = tt.args.initCfg
			errs := ini.initializeBatchPlaywright()
			if !reflect.DeepEqual(ini.cfg, tt.want) {
				t.Errorf("initializeBatchPlaywright() got = %v, want %v", ini.cfg, tt.want)
			}
			if !reflect.DeepEqual(errs, tt.wantErrs) {
				t.Errorf("initializeBatchPlaywright() got = %v, want %v", errs, tt.wantErrs)
			}
		})
	}
}

func Test_initializer_initializeBatchXcuitest(t *testing.T) {
	dir := fs.NewDir(t, "apps",
		fs.WithFile("ios-app.ipa", "myAppContent", fs.WithMode(0644)),
		fs.WithDir("ios-folder-app.app", fs.WithMode(0755)))
	defer dir.Remove()

	ini := &initializer{
		cfg: &initConfig{},
	}
	var emptyErr []error

	type args struct {
		initCfg *initConfig
		flags   func() *pflag.FlagSet
	}
	tests := []struct {
		name     string
		args     args
		want     *initConfig
		wantErrs []error
	}{
		{
			name: "Basic",
			args: args{
				flags: func() *pflag.FlagSet {
					var deviceFlag flags.Device
					p := pflag.NewFlagSet("tests", pflag.ContinueOnError)
					p.Var(&deviceFlag, "device", "")
					err := p.Parse([]string{`--device`, `name=iPhone .*`})
					if err != nil {
						t.Errorf("failed to parse test args: %v", err)
					}
					return p
				},
				initCfg: &initConfig{
					frameworkName: xcuitest.Kind,
					app:           dir.Join("ios-app.ipa"),
					testApp:       dir.Join("ios-app.ipa"),
					deviceFlag: flags.Device{
						Changed: true,
						Device: config.Device{
							Name: "iPhone .*",
						},
					},
					device: config.Device{
						Name: "iPhone .*",
					},
					region:       "us-west-1",
					artifactWhen: "fail",
				},
			},
			want: &initConfig{
				frameworkName: xcuitest.Kind,
				app:           dir.Join("ios-app.ipa"),
				testApp:       dir.Join("ios-app.ipa"),
				deviceFlag: flags.Device{
					Changed: true,
					Device: config.Device{
						Name: "iPhone .*",
					},
				},
				device: config.Device{
					Name: "iPhone .*",
				},
				region:       "us-west-1",
				artifactWhen: config.WhenFail,
			},
			wantErrs: emptyErr,
		},
		{
			name: "invalid download config",
			args: args{
				flags: func() *pflag.FlagSet {
					var deviceFlag flags.Device
					p := pflag.NewFlagSet("tests", pflag.ContinueOnError)
					p.Var(&deviceFlag, "device", "")
					return p
				},
				initCfg: &initConfig{
					frameworkName:   xcuitest.Kind,
					artifactWhenStr: "dummy",
				},
			},
			want: &initConfig{
				frameworkName:   xcuitest.Kind,
				artifactWhenStr: "dummy",
			},
			wantErrs: []error{
				errors.New("no app provided"),
				errors.New("no testApp provided"),
				errors.New("either device or simulator configuration needs to be provided"),
				errors.New("dummy: unknown download condition"),
			},
		},
		{
			name: "no flags",
			args: args{
				flags: func() *pflag.FlagSet {
					var deviceFlag flags.Device
					p := pflag.NewFlagSet("tests", pflag.ContinueOnError)
					p.Var(&deviceFlag, "device", "")
					return p
				},
				initCfg: &initConfig{
					frameworkName: xcuitest.Kind,
				},
			},
			want: &initConfig{
				frameworkName: xcuitest.Kind,
			},
			wantErrs: []error{
				errors.New("no app provided"),
				errors.New("no testApp provided"),
				errors.New("either device or simulator configuration needs to be provided"),
			},
		},
		{
			name: "invalid app file / test file",
			args: args{
				flags: func() *pflag.FlagSet {
					p := pflag.NewFlagSet("tests", pflag.ContinueOnError)
					return p
				},
				initCfg: &initConfig{
					frameworkName: xcuitest.Kind,
					app:           dir.Join("truc", "ios-app.ipa"),
					testApp:       dir.Join("truc", "ios-app.ipa"),
				},
			},
			want: &initConfig{
				frameworkName: xcuitest.Kind,
				app:           dir.Join("truc", "ios-app.ipa"),
				testApp:       dir.Join("truc", "ios-app.ipa"),
			},
			wantErrs: []error{
				errors.New("either device or simulator configuration needs to be provided"),
				fmt.Errorf("app: %s: stat %s: no such file or directory", dir.Join("truc", "ios-app.ipa"), dir.Join("truc", "ios-app.ipa")),
				fmt.Errorf("testApp: %s: stat %s: no such file or directory", dir.Join("truc", "ios-app.ipa"), dir.Join("truc", "ios-app.ipa")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ini.cfg = tt.args.initCfg
			errs := ini.initializeBatchXcuitest(tt.args.flags())
			if !reflect.DeepEqual(ini.cfg, tt.want) {
				t.Errorf("initializeBatchXcuitest() got = %v, want %v", ini.cfg, tt.want)
			}
			if !reflect.DeepEqual(errs, tt.wantErrs) {
				t.Errorf("initializeBatchXcuitest() got = %v, want %v", errs, tt.wantErrs)
			}
		})
	}
}

func Test_initializer_initializeBatchEspresso(t *testing.T) {
	dir := fs.NewDir(t, "apps",
		fs.WithFile("android-app.apk", "myAppContent", fs.WithMode(0644)))
	defer dir.Remove()

	ini := &initializer{
		cfg: &initConfig{},
	}
	var emptyErr []error

	type args struct {
		initCfg *initConfig
		flags   func() *pflag.FlagSet
	}
	tests := []struct {
		name     string
		args     args
		want     *initConfig
		wantErrs []error
	}{
		{
			name: "Basic",
			args: args{
				flags: func() *pflag.FlagSet {
					var deviceFlag flags.Device
					p := pflag.NewFlagSet("tests", pflag.ContinueOnError)
					p.Var(&deviceFlag, "device", "")
					err := p.Parse([]string{`--device`, `name=HTC .*`})
					if err != nil {
						t.Errorf("failed to parse test args: %v", err)
					}
					return p
				},
				initCfg: &initConfig{
					frameworkName: espresso.Kind,
					app:           dir.Join("android-app.apk"),
					testApp:       dir.Join("android-app.apk"),
					deviceFlag: flags.Device{
						Changed: true,
						Device: config.Device{
							Name: "HTC .*",
						},
					},
					region:       "us-west-1",
					artifactWhen: "fail",
				},
			},
			want: &initConfig{
				frameworkName: espresso.Kind,
				app:           dir.Join("android-app.apk"),
				testApp:       dir.Join("android-app.apk"),
				deviceFlag: flags.Device{
					Changed: true,
					Device: config.Device{
						Name: "HTC .*",
					},
				},
				device: config.Device{
					Name: "HTC .*",
				},
				region:       "us-west-1",
				artifactWhen: config.WhenFail,
			},
			wantErrs: emptyErr,
		},
		{
			name: "invalid download config",
			args: args{
				flags: func() *pflag.FlagSet {
					var deviceFlag flags.Device
					p := pflag.NewFlagSet("tests", pflag.ContinueOnError)
					p.Var(&deviceFlag, "device", "")
					return p
				},
				initCfg: &initConfig{
					frameworkName:   espresso.Kind,
					artifactWhenStr: "dummy",
				},
			},
			want: &initConfig{
				frameworkName:   espresso.Kind,
				artifactWhenStr: "dummy",
			},
			wantErrs: []error{
				errors.New("no app provided"),
				errors.New("no testApp provided"),
				errors.New("either device or emulator configuration needs to be provided"),
				errors.New("dummy: unknown download condition"),
			},
		},
		{
			name: "no flags",
			args: args{
				flags: func() *pflag.FlagSet {
					var deviceFlag flags.Device
					p := pflag.NewFlagSet("tests", pflag.ContinueOnError)
					p.Var(&deviceFlag, "device", "")
					return p
				},
				initCfg: &initConfig{
					frameworkName: espresso.Kind,
				},
			},
			want: &initConfig{
				frameworkName: espresso.Kind,
			},
			wantErrs: []error{
				errors.New("no app provided"),
				errors.New("no testApp provided"),
				errors.New("either device or emulator configuration needs to be provided"),
			},
		},
		{
			name: "invalid app file / test file",
			args: args{
				flags: func() *pflag.FlagSet {
					var deviceFlag flags.Device
					p := pflag.NewFlagSet("tests", pflag.ContinueOnError)
					p.Var(&deviceFlag, "device", "")
					return p
				},
				initCfg: &initConfig{
					frameworkName: espresso.Kind,
					app:           dir.Join("truc", "android-app.apk"),
					testApp:       dir.Join("truc", "android-app.apk"),
				},
			},
			want: &initConfig{
				frameworkName: espresso.Kind,
				app:           dir.Join("truc", "android-app.apk"),
				testApp:       dir.Join("truc", "android-app.apk"),
			},
			wantErrs: []error{
				errors.New("either device or emulator configuration needs to be provided"),
				fmt.Errorf("app: %s: stat %s: no such file or directory", dir.Join("truc", "android-app.apk"), dir.Join("truc", "android-app.apk")),
				fmt.Errorf("testApp: %s: stat %s: no such file or directory", dir.Join("truc", "android-app.apk"), dir.Join("truc", "android-app.apk")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ini.cfg = tt.args.initCfg
			errs := ini.initializeBatchEspresso(tt.args.flags())
			if !reflect.DeepEqual(ini.cfg, tt.want) {
				t.Errorf("initializeBatchEspresso() got = %v, want %v", ini.cfg, tt.want)
			}
			if !reflect.DeepEqual(errs, tt.wantErrs) {
				t.Errorf("initializeBatchEspresso() got = %v, want %v", errs, tt.wantErrs)
			}
		})
	}
}

func Test_initializer_initializeBatchImageRunner(t *testing.T) {
	ini := &initializer{
		infoReader: &mocks.FakeFrameworkInfoReader{VersionsFn: func(context.Context, string) ([]framework.Metadata, error) {
			return []framework.Metadata{
				{
					FrameworkName:    "imagerunner",
					FrameworkVersion: "",
				},
			}, nil
		}},
		userService: &mocks.UserService{ConcurrencyFn: func(context.Context) (iam.Concurrency, error) {
			return iam.Concurrency{
				Org: iam.OrgConcurrency{
					Allowed: iam.CloudConcurrency{
						VDC: 2,
					},
				},
			}, nil
		}},
		cfg: &initConfig{},
	}
	var emptyErr []error

	type args struct {
		initCfg *initConfig
	}
	tests := []struct {
		name     string
		args     args
		want     *initConfig
		wantErrs []error
	}{
		{
			name: "Basic",
			args: args{
				initCfg: &initConfig{
					frameworkName: "imagerunner",
					dockerImage:   "ubuntu:latest",
					region:        "us-west-1",
					artifactWhen:  "fail",
				},
			},
			want: &initConfig{
				frameworkName: "imagerunner",
				dockerImage:   "ubuntu:latest",
				region:        "us-west-1",
				artifactWhen:  config.WhenFail,
			},
			wantErrs: emptyErr,
		},
		{
			name: "invalid browser/platform",
			args: args{
				initCfg: &initConfig{
					frameworkName:   "imagerunner",
					dockerImage:     "ubuntu::buggy",
					artifactWhenStr: "dummy",
				},
			},
			want: &initConfig{
				frameworkName:   "imagerunner",
				dockerImage:     "ubuntu::buggy",
				artifactWhenStr: "dummy",
			},
			wantErrs: []error{
				errors.New("dockerImage: ubuntu::buggy is not a valid docker image"),
				errors.New("dummy: unknown download condition"),
			},
		},
		{
			name: "no flags",
			args: args{
				initCfg: &initConfig{
					frameworkName: "imagerunner",
				},
			},
			want: &initConfig{
				frameworkName: "imagerunner",
			},
			wantErrs: []error{
				errors.New(msg.MissingDockerImage),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ini.cfg = tt.args.initCfg
			errs := ini.initializeBatchImageRunner()
			if !reflect.DeepEqual(ini.cfg, tt.want) {
				t.Errorf("initializeBatchImageRunner() got = %v, want %v", ini.cfg, tt.want)
			}
			if !reflect.DeepEqual(errs, tt.wantErrs) {
				t.Errorf("initializeBatchImageRunner() got = %v, want %v", errs, tt.wantErrs)
			}
		})
	}
}
