//go:build unix

package ansipixels

import (
	"os"
	"syscall"
)

var signalList = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGWINCH}

func (ap *AnsiPixels) IsResizeSignal(s os.Signal) bool {
	return s == syscall.SIGWINCH
}
