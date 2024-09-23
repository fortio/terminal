//go:build !unix
// +build !unix

package terminal

import (
	"os"
	"time"
)

const IsUnix = false

func TimeoutToTimeval(_ time.Duration) any {
	return nil
}

func TimeoutReader(_ int, _ any, buf []byte) (int, error) {
	return os.Stdin.Read(buf)
}
