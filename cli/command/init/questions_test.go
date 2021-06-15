package init

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"gotest.tools/v3/fs"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/hinshun/vt10x"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/devices"
	"github.com/saucelabs/saucectl/internal/framework"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/vmd"
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
	ir := &mocks.FakeFrameworkInfoReader{
		FrameworkResponse: []framework.Framework{{Name: "cypress"}, {Name: "espresso"}, {Name: "playwright"}},
	}
	testCases := []questionTest{
		{
			name:      "Default",
			procedure: stringToProcedure("✓🔚"),
			ini:       &initiator{infoReader: ir},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askFramework(cfg)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{frameworkName: "cypress"},
		},
		{
			name:      "Type In",
			procedure: stringToProcedure("esp✓🔚"),
			ini:       &initiator{infoReader: ir},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askFramework(cfg)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{frameworkName: "espresso"},
		},
		{
			name:      "Arrow In",
			procedure: stringToProcedure("↓✓🔚"),
			ini:       &initiator{infoReader: ir},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askFramework(cfg)
			},
			startState:    &initConfig{},
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
			name:      "Default",
			procedure: stringToProcedure("✓🔚"),
			ini:       &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askRegion(cfg)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{region: region.USWest1.String()},
		},
		{
			name:      "Type US",
			procedure: stringToProcedure("us-✓🔚"),
			ini:       &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askRegion(cfg)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{region: region.USWest1.String()},
		},
		{
			name:      "Type EU",
			procedure: stringToProcedure("eu-✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askRegion(cfg)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{region: region.EUCentral1.String()},
		},
		{
			name:      "Select EU",
			procedure: stringToProcedure("↓✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askRegion(cfg)
			},
			startState:    &initConfig{},
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
			ini:       &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDownloadWhen(cfg)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{artifactWhen: config.WhenFail},
		},
		{
			name:      "Second is pass",
			procedure: stringToProcedure("↓✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDownloadWhen(cfg)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{artifactWhen: config.WhenPass},
		},
		{
			name:      "Type always",
			procedure: stringToProcedure("alw✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDownloadWhen(cfg)
			},
			startState:    &initConfig{},
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
			ini:       &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDevice(cfg, devs)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{device: config.Device{Name: "Google Pixel 3"}},
		},
		{
			name:      "Input is captured",
			procedure: stringToProcedure("Pixel 4✓🔚"),
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askDevice(cfg, devs)
			},
			startState:    &initConfig{},
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
		{Name: "Google Pixel 3 Emulator"},
		{Name: "Google Pixel 4 Emulator"},
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
				return i.askEmulator(cfg, vmds)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{emulator: config.Emulator{Name: "Google Pixel 3 Emulator"}},
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
				return i.askEmulator(cfg, vmds)
			},
			startState:    &initConfig{},
			expectedState: &initConfig{emulator: config.Emulator{Name: "Google Pixel 4 Emulator"}},
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
			FrameworkName:    "testcafe",
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
			FrameworkName:    "testcafe",
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
				_, err := c.ExpectString("Select platform")
				if err != nil {
					return err
				}
				_, err = c.SendLine("Windows 10")
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
			ini: &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askPlatform(cfg, metas)
			},
			startState:    &initConfig{frameworkName: "testcafe", frameworkVersion: "1.5.0"},
			expectedState: &initConfig{frameworkName: "testcafe", frameworkVersion: "1.5.0", browserName: "googlechrome", mode: "sauce", platformName: "Windows 10"},
		},
		{
			name: "macOS",
			procedure: func(c *expect.Console) error {
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
			ini: &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askPlatform(cfg, metas)
			},
			startState:    &initConfig{frameworkName: "testcafe", frameworkVersion: "1.5.0"},
			expectedState: &initConfig{frameworkName: "testcafe", frameworkVersion: "1.5.0", platformName: "macOS 11.00", browserName: "firefox", mode: "sauce"},
		},
		{
			name: "docker",
			procedure: func(c *expect.Console) error {
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
			ini: &initiator{},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askPlatform(cfg, metas)
			},
			startState:    &initConfig{frameworkName: "testcafe", frameworkVersion: "1.5.0"},
			expectedState: &initConfig{frameworkName: "testcafe", frameworkVersion: "1.5.0", platformName: "", browserName: "chrome", mode: "docker"},
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
			FrameworkName:    "testcafe",
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
			FrameworkName:    "testcafe",
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
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askVersion(cfg, metas)
			},
			startState:    &initConfig{frameworkName: "testcafe"},
			expectedState: &initConfig{frameworkName: "testcafe", frameworkVersion: "1.5.0"},
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
			ini: &initiator{
				infoReader: &mocks.FakeFrameworkInfoReader{},
			},
			execution: func(i *initiator, cfg *initConfig) error {
				return i.askVersion(cfg, metas)
			},
			startState:    &initConfig{frameworkName: "testcafe"},
			expectedState: &initConfig{frameworkName: "testcafe", frameworkVersion: "1.3.0"},
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
			startState:    &initConfig{},
			expectedState: &initConfig{app: dir.Join("android-app.apk")},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}

func Test_frameworkSpecificSettings(t *testing.T) {
	type args struct {
		framework string
	}
	type want struct {
		nativeFramework  bool
		needsApps        bool
		needsCypressJson bool
		needsDevice      bool
		needsEmulator    bool
		needsPlatform    bool
		needsRootDir     bool
		needsVersion     bool
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "espresso",
			args: args{
				"espresso",
			},
			want: want{
				nativeFramework: true,
				needsApps:       true,
				needsEmulator:   true,
				needsDevice:     true,
			},
		},
		{
			name: "xcuitest",
			args: args{
				"xcuitest",
			},
			want: want{
				nativeFramework: true,
				needsApps:       true,
				needsDevice:     true,
			},
		},
		{
			name: "cypress",
			args: args{
				"cypress",
			},
			want: want{
				needsVersion:     true,
				needsRootDir:     true,
				needsPlatform:    true,
				needsCypressJson: true,
			},
		},
		{
			name: "testcafe",
			args: args{
				"testcafe",
			},
			want: want{
				needsVersion:  true,
				needsRootDir:  true,
				needsPlatform: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNativeFramework(tt.args.framework); got != tt.want.nativeFramework {
				t.Errorf("isNativeFramework() = %v, want %v", got, tt.want)
			}
			if got := needsCypressJson(tt.args.framework); got != tt.want.needsCypressJson {
				t.Errorf("needsCypressJson() = %v, want %v", got, tt.want)
			}
			if got := needsVersion(tt.args.framework); got != tt.want.needsVersion {
				t.Errorf("needsVersion() = %v, want %v", got, tt.want)
			}
			if got := needsPlatform(tt.args.framework); got != tt.want.needsPlatform {
				t.Errorf("needsPlatform() = %v, want %v", got, tt.want)
			}
			if got := needsDevice(tt.args.framework); got != tt.want.needsDevice {
				t.Errorf("needsDevice() = %v, want %v", got, tt.want)
			}
			if got := needsEmulator(tt.args.framework); got != tt.want.needsEmulator {
				t.Errorf("needsEmulator() = %v, want %v", got, tt.want)
			}
			if got := needsRootDir(tt.args.framework); got != tt.want.needsRootDir {
				t.Errorf("needsRootDir() = %v, want %v", got, tt.want)
			}
			if got := needsApps(tt.args.framework); got != tt.want.needsApps {
				t.Errorf("needsApps() = %v, want %v", got, tt.want)
			}
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
			FrameworkName:    "cypress",
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
		VersionsResponse:  frameworkVersions,
		FrameworkResponse: []framework.Framework{{Name: "cypress"}, {Name: "espresso"}},
	}
	dr := &mocks.FakeDevicesReader{
		GetDevicesFn: func(ctx context.Context, s string) ([]devices.Device, error) {
			return []devices.Device{
				{Name: "Google Pixel 3"},
				{Name: "Google Pixel 4"},
			}, nil
		},
	}
	er := &mocks.FakeEmulatorsReader{
		GetVirtualDevicesFn: func(ctx context.Context, s string) ([]vmd.VirtualDevice, error) {
			return []vmd.VirtualDevice{
				{Name: "Google Pixel Emulator"},
				{Name: "Samsung Galaxy Emulator"},
			}, nil
		},
	}

	testCases := []questionTest{
		{
			name: "Complete Configuration (espresso)",
			procedure: func(c *expect.Console) error {
				c.ExpectString("Select framework")
				c.SendLine("espresso")
				c.ExpectString("Select region")
				c.SendLine("us-west-1")
				c.ExpectString("Application to test")
				c.SendLine(dir.Join("android-app.apk"))
				c.ExpectString("Test application")
				c.SendLine(dir.Join("android-app.apk"))
				c.ExpectString("Select device pattern:")
				c.SendLine("Google Pixel .*")
				c.ExpectString("Select emulator:")
				c.SendLine("Google Pixel Emulator")
				c.ExpectString("Download artifacts:")
				c.SendLine("when tests are passing")
				c.ExpectEOF()
				return nil
			},
			ini: &initiator{infoReader: ir, deviceReader: dr, vmdReader: er},
			execution: func(i *initiator, cfg *initConfig) error {
				newCfg, err := i.configure()
				if err != nil {
					return err
				}
				*cfg = *newCfg
				return nil
			},
			startState: &initConfig{},
			expectedState: &initConfig{
				frameworkName: "espresso",
				app:           dir.Join("android-app.apk"),
				testApp:       dir.Join("android-app.apk"),
				emulator:      config.Emulator{Name: "Google Pixel Emulator"},
				device:        config.Device{Name: "Google Pixel .*"},
				region:        "us-west-1",
				artifactWhen:  config.WhenPass,
			},
		},
		{
			name: "Complete Configuration (cypress)",
			procedure: func(c *expect.Console) error {
				c.ExpectString("Select framework")
				c.SendLine("cypress")
				c.ExpectString("Select region")
				c.SendLine("us-west-1")
				c.ExpectString("Select cypress version")
				c.SendLine("7.5.0")
				c.ExpectString("Cypress configuration file:")
				c.SendLine(dir.Join("cypress.json"))
				c.ExpectString("Select platform:")
				c.SendLine("Windows 10")
				c.ExpectString("Select Browser:")
				c.SendLine("chrome")
				c.ExpectString("Download artifacts:")
				c.SendLine("when tests are passing")
				c.ExpectEOF()
				return nil
			},
			ini: &initiator{infoReader: ir, deviceReader: dr, vmdReader: er},
			execution: func(i *initiator, cfg *initConfig) error {
				newCfg, err := i.configure()
				if err != nil {
					return err
				}
				*cfg = *newCfg
				return nil
			},
			startState: &initConfig{},
			expectedState: &initConfig{
				frameworkName:    "cypress",
				frameworkVersion: "7.5.0",
				cypressJson:      dir.Join("cypress.json"),
				platformName:     "windows 10",
				browserName:      "googlechrome",
				mode:             "sauce",
				region:           "us-west-1",
				artifactWhen:     config.WhenPass,
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(lt *testing.T) {
			executeQuestionTestWithTimeout(lt, tt)
		})
	}
}
