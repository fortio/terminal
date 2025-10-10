//go:build unix && !test_alt_timeoutreader
// +build unix,!test_alt_timeoutreader

package terminal

import (
	"errors"
	"io"
	"os"
	"syscall"
	"time"

	"fortio.org/log"
	"fortio.org/safecast"
	"golang.org/x/sys/unix"
)

const IsUnix = true

func TimeoutToTimeval(timeout time.Duration) *unix.Timeval {
	tv := unix.NsecToTimeval(timeout.Nanoseconds())
	return &tv
}

func ReadWithTimeout(fd int, tv *unix.Timeval, buf []byte) (int, error) {
	var readfds unix.FdSet
	readfds.Set(fd)
	n, err := unix.Select(fd+1, &readfds, nil, nil, tv)
	if errors.Is(err, syscall.EINTR) {
		log.LogVf("Interrupted select")
		return 0, nil
	}
	if err != nil {
		log.Errf("Select error: %v", err)
		return 0, err
	}
	if n == 0 {
		return 0, nil // timeout case
	}
	n, err = unix.Read(fd, buf)
	if n == 0 && err == nil {
		err = io.EOF
	}
	return n, err
}

type TimeoutReader struct {
	fd       int
	tv       *unix.Timeval
	blocking bool     // true if the reader is blocking (timeout == 0), false if it has a timeout set
	ostream  *os.File // original file/stream
}

func NewTimeoutReader(stream *os.File, timeout time.Duration) *TimeoutReader {
	if timeout < 0 {
		panic("Timeout must be greater or equal to 0")
	}
	return &TimeoutReader{
		fd:       safecast.MustConv[int](stream.Fd()),
		tv:       TimeoutToTimeval(timeout),
		blocking: timeout == 0,
		ostream:  stream,
	}
}

func (tr *TimeoutReader) Read(buf []byte) (int, error) {
	if tr.blocking {
		return tr.ostream.Read(buf)
	}
	return ReadWithTimeout(tr.fd, tr.tv, buf)
}

func (tr *TimeoutReader) ReadBlocking(buf []byte) (int, error) {
	return tr.ostream.Read(buf)
}

func (tr *TimeoutReader) ReadImmediate(buf []byte) (int, error) {
	if tr.blocking {
		return tr.ostream.Read(buf)
	}
	var zeroTv unix.Timeval
	return ReadWithTimeout(tr.fd, &zeroTv, buf)
}

// ChangeTimeout on unix should be called from same goroutine as any Read* or not concurrently.
func (tr *TimeoutReader) ChangeTimeout(timeout time.Duration) {
	if tr.blocking && timeout > 0 {
		panic("Cannot change from blocking to non-blocking mode")
	}
	tr.tv = TimeoutToTimeval(timeout)
}

// Close closes the underlying stream if we are in blocking mode.
// nop otherwise.
func (tr *TimeoutReader) Close() (err error) {
	if tr.blocking && tr.ostream != nil {
		err = tr.ostream.Close()
		tr.ostream = nil // Clear the stream reference
	}
	return err
}

// IsClosed returns true if Close() has been called (and for the other implementation a new one should be created).
// Always false on unix/select mode because we can keep using it forever, unlike the goroutine based one.
// Unless we are in blocking mode and Close() was called.
func (tr *TimeoutReader) IsClosed() bool {
	return tr.ostream == nil
}
