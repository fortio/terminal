// This was really `windows` but wasm also has SIGTERM so let's see what breaks.
//go:build !unix
// +build !unix

package ansipixels

import (
	"os"
	"syscall"
)

var signalList = []os.Signal{os.Interrupt, syscall.SIGTERM}

func (ap *AnsiPixels) IsResizeSignal(s os.Signal) bool { return false }
