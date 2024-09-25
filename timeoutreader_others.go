//go:build !unix
// +build !unix

package terminal

import (
	"os"
	"time"
)

const IsUnix = false

type TimeoutReader struct {
	file *os.File
}

func NewTimeoutReader(stream *os.File, _ time.Duration) *TimeoutReader {
	return &TimeoutReader{
		file: stream,
	}
}

func (tr *TimeoutReader) Read(buf []byte) (int, error) {
	return tr.file.Read(buf)
}
