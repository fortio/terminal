//go:build windows
// +build windows

package ansipixels

import (
	"os"
	"syscall"
)

var signalList = []os.Signal{os.Interrupt, syscall.SIGTERM}

func (ap *AnsiPixels) IsResizeSignal(s os.Signal) bool { return false }
