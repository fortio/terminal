//go:build unix

package ansipixels

import (
	"syscall"
)

const ResizeSignal = syscall.SIGWINCH
