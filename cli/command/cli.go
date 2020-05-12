package command

import (
	"github.com/docker/docker/pkg/term"
	"github.com/saucelabs/saucectl/cli/streams"
	"io"
)

// SauceCtlCli is the cli context
type SauceCtlCli struct {
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
	stdin, stdout, stderr := term.StdStreams()
	cli := &SauceCtlCli{
		in:     streams.NewIn(stdin),
		out:    streams.NewOut(stdout),
		err:    stderr,
	}

	return cli, nil
}
