package terminal

import (
	"errors"
	"io"
	"syscall"
	"time"

	"fortio.org/log"
	"golang.org/x/sys/unix"
)

func TimeoutToTimeval(timeout time.Duration) *unix.Timeval {
	tv := unix.NsecToTimeval(timeout.Nanoseconds())
	return &tv
}

func TimeoutReader(fd int, tv *unix.Timeval, buf []byte) (int, error) {
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
