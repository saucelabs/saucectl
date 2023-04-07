package streams

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/moby/term"
)

// Streams is an interface which exposes the standard input and output streams
type Streams interface {
	In() *In
	Out() *Out
	Err() io.Writer
}

// commonStream is an input stream used by the DockerCli to read user input
type commonStream struct {
	fd         uintptr
	isTerminal bool
	state      *term.State
}

// FD returns the file descriptor number for this stream
func (s *commonStream) FD() uintptr {
	return s.fd
}

// IsTerminal returns true if this stream is connected to a terminal
func (s *commonStream) IsTerminal() bool {
	return s.isTerminal
}

// An IOStreamer handles copying input to and output from streams to the
// connection.
type IOStreamer struct {
	Streams      Streams
	InputStream  io.ReadCloser
	OutputStream io.Writer
	ErrorStream  io.Writer

	Resp types.HijackedResponse
}

// Stream handles setting up the IO and then begins streaming stdin/stdout
// to/from the hijacked connection, blocking until it is either done reading
// output, the user inputs the detach key sequence when in TTY mode, or when
// the given context is cancelled.
func (h *IOStreamer) Stream(ctx context.Context) error {
	outputDone := h.beginOutputStream()
	inputDone, detached := h.beginInputStream()

	select {
	case err := <-outputDone:
		return err
	case <-inputDone:
		// Input stream has closed.
		if h.OutputStream != nil || h.ErrorStream != nil {
			// Wait for output to complete streaming.
			select {
			case err := <-outputDone:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	case err := <-detached:
		// Got a detach key sequence.
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *IOStreamer) beginOutputStream() <-chan error {
	outputDone := make(chan error)
	go func() {
		var err error

		// When TTY is ON, use regular copy
		// if h.OutputStream != nil {
		// 	_, err = io.Copy(h.OutputStream, h.Resp.Reader)
		// } else {
		_, err = stdcopy.StdCopy(h.OutputStream, h.ErrorStream, h.Resp.Reader)
		// }

		if err != nil {
			fmt.Printf("Error receiveStdout: %s", err)
		}

		outputDone <- err
	}()

	return outputDone
}

func (h *IOStreamer) beginInputStream() (doneC <-chan struct{}, detachedC <-chan error) {
	inputDone := make(chan struct{})
	detached := make(chan error)

	go func() {
		if h.InputStream != nil {
			_, err := io.Copy(h.Resp.Conn, h.InputStream)

			if err != nil {
				// This error will also occur on the receive
				// side (from stdout) where it will be
				// propagated back to the caller.
				fmt.Printf("Error sendStdin: %s", err)
			}
		}

		if err := h.Resp.CloseWrite(); err != nil {
			fmt.Printf("Couldn't send EOF: %s", err)
		}

		close(inputDone)
	}()

	return inputDone, detached
}
