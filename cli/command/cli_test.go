package command

import (
	"testing"

	"github.com/docker/docker/pkg/term"
	"github.com/saucelabs/saucectl/cli/streams"
	"github.com/stretchr/testify/assert"
)

func TestRootCmd(t *testing.T) {
	cli := NewSauceCtlCli()
	assert.Equal(t, cli.In(), cli.in)
	assert.Equal(t, cli.Out(), cli.out)
	assert.Equal(t, cli.Err(), cli.err)

	stdin, _, _ := term.StdStreams()
	newIn := streams.NewIn(stdin)
	cli.SetIn(newIn)
	assert.Equal(t, cli.In(), newIn)
}
