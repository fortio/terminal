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
	fd int
	tv *unix.Timeval
}

func NewTimeoutReader(stream *os.File, timeout time.Duration) *TimeoutReader {
	if timeout <= 0 {
		panic("Timeout must be greater than 0")
	}
	return &TimeoutReader{
		fd: safecast.MustConvert[int](stream.Fd()),
		tv: TimeoutToTimeval(timeout),
	}
}

func (tr *TimeoutReader) Read(buf []byte) (int, error) {
	return ReadWithTimeout(tr.fd, tr.tv, buf)
}

// ChangeTimeout on unix should be called from same goroutine as any Read* or not concurrently.
func (tr *TimeoutReader) ChangeTimeout(timeout time.Duration) {
	tr.tv = TimeoutToTimeval(timeout)
}

// We don't really close the underlying but this is a chance to cleanup for the other implementation.
func (tr *TimeoutReader) Close() error {
	return nil
}

// IsClosed returns true if Close() has been called (and for the other implementation a new one should be created).
// Always false on unix/select mode because we can keep using it forever, unlike the goroutine based one.
func (tr *TimeoutReader) IsClosed() bool {
	return false
}
