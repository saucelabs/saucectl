package command

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/docker/docker/pkg/term"
	"github.com/rs/zerolog"
	"github.com/saucelabs/saucectl/cli/streams"
	"github.com/saucelabs/saucectl/cli/version"
	"github.com/tj/go-update"
	"github.com/tj/go-update/progress"
	"github.com/tj/go-update/stores/github"
	"github.com/tj/survey"
)

// SauceCtlCli is the cli context
type SauceCtlCli struct {
	Logger zerolog.Logger

	in  *streams.In
	out *streams.Out
	err io.Writer
}

// Out returns the writer used for stdout
func (cli *SauceCtlCli) Out() *streams.Out {
	return cli.out
}

// Err returns the writer used for stderr
func (cli *SauceCtlCli) Err() io.Writer {
	return cli.err
}

// SetIn sets the reader used for stdin
func (cli *SauceCtlCli) SetIn(in *streams.In) {
	cli.in = in
}

// In returns the reader used for stdin
func (cli *SauceCtlCli) In() *streams.In {
	return cli.in
}

// NewSauceCtlCli creates the context object for the cli
func NewSauceCtlCli() (*SauceCtlCli, error) {
	// UNIX Time is faster and smaller than most timestamps
	// If you set zerolog.TimeFieldFormat to an empty string,
	// logs will write with UNIX time
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger.Info().Msg("Start Program")

	stdin, stdout, stderr := term.StdStreams()
	cli := &SauceCtlCli{
		Logger: logger,
		in:     streams.NewIn(stdin),
		out:    streams.NewOut(stdout),
		err:    stderr,
	}

	// check if new version is available
	if err := checkUpdates(cli); err != nil {
		cli.Logger.Err(err)
		panic(err)
	}

	return cli, nil
}

func checkUpdates(cli *SauceCtlCli) error {
	doUpdate := false
	m := &update.Manager{
		Command: "saucectl-internal",
		Store: &github.Store{
			Owner:   "saucelabs",
			Repo:    "saucectl",
			Version: version.Version,
		},
	}

	// fetch the new releases
	releases, err := m.LatestReleases()
	if err != nil {
		return err
	}

	// no updates
	if len(releases) == 0 {
		cli.Logger.Info().Msg("No updates, continuing program")
		return nil
	}

	// latest release
	latest := releases[0]

	qs := &survey.Confirm{
		Message: fmt.Sprintf("A new saucectl version was found (%s), do you want to update?", latest.Version),
	}
	survey.AskOne(qs, &doUpdate, nil)

	if !doUpdate {
		fmt.Println()
		return nil
	}

	// find the tarball for this system
	os := "linux"
	switch runtime.GOOS {
	case "darwin":
		os = "mac"
	case "windows":
		os = "win"
	}
	arch := "64-bit"
	switch runtime.GOARCH {
	case "386":
		os = "32-bit"
	}
	a := latest.FindTarball(os, arch)
	if a == nil {
		cli.Logger.Info().Msgf("no binary for your system (os: %s, arch: %s)", runtime.GOOS, runtime.GOARCH)
		return nil
	}

	// whitespace
	fmt.Println()

	// download tarball to a tmp dir
	tarball, err := a.DownloadProxy(progress.Reader)
	if err != nil {
		return err
	}

	// install it
	if err := m.Install(tarball); err != nil {
		return err
	}

	cli.Logger.Info().Msgf("Updated to %s\n", latest.Version)
	return nil
}
