package command

import (
	"io"
	"os"

	"github.com/docker/docker/pkg/term"
	"github.com/rs/zerolog"
	"github.com/saucelabs/saucectl/cli/streams"
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
	// zerolog.SetGlobalLevel(zerolog.ErrorLevel)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger.Info().Msg("Start Program")

	stdin, stdout, stderr := term.StdStreams()
	cli := &SauceCtlCli{
		Logger: logger,
		in:     streams.NewIn(stdin),
		out:    streams.NewOut(stdout),
		err:    stderr,
	}

	return cli, nil
}
