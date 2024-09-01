package terminal

import (
	"os"
	"time"
)

func TimeoutToTimeval(timeout time.Duration) any {
	return nil
}

func TimeoutReader(fd int, tv any, buf []byte) (int, error) {
	return os.Stdin.Read(buf)
}
