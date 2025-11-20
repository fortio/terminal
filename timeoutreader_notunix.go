//go:build !unix || test_alt_timeoutreader

// To test on unix/mac use for instance:
// make GO_BUILD_TAGS=test_alt_timeoutreader,no_net,no_json,no_pprof

package terminal

import (
	"io"
	"time"
)

const IsUnix = false

type SystemTimeoutReader = TimeoutReader

func NewSystemTimeoutReader(stream io.Reader, timeout time.Duration) *TimeoutReader {
	return NewTimeoutReader(stream, timeout)
}
