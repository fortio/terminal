package terminal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"fortio.org/log"
)

type InterruptReader struct {
	reader  io.Reader // stdin typically
	fd      int
	buf     []byte
	reset   []byte // original buffer start
	bufSize int
	err     error
	mu      sync.Mutex
	cond    sync.Cond
	cancel  context.CancelFunc
}

var ErrUserInterrupt = NewErrInterrupted("terminal interrupted by user")

type InterruptedError struct {
	DetailedReason string
	OriginalError  error
}

func (e InterruptedError) Unwrap() error {
	return e.OriginalError
}

func (e InterruptedError) Error() string {
	if e.OriginalError != nil {
		return "terminal interrupted: " + e.DetailedReason + ": " + e.OriginalError.Error()
	}
	return "terminal interrupted: " + e.DetailedReason
}

func NewErrInterrupted(reason string) InterruptedError {
	return InterruptedError{DetailedReason: reason}
}

func NewErrInterruptedWithErr(reason string, err error) InterruptedError {
	return InterruptedError{DetailedReason: reason, OriginalError: err}
}

// NewInterruptReader creates a new interrupt reader.
// it needs to be Start()ed to start reading from the underlying reader
// and intercept Ctrl-C and listen for interrupt signals.
func NewInterruptReader(reader *os.File, bufSize int) *InterruptReader {
	ir := &InterruptReader{
		reader:  reader,
		bufSize: bufSize,
		buf:     make([]byte, 0, bufSize),
		fd:      int(reader.Fd()), //nolint:gosec // it's on almost all platforms.
	}
	ir.reset = ir.buf
	ir.cond = *sync.NewCond(&ir.mu)
	log.Config.GoroutineID = true
	return ir
}

// Start or restart (after a cancel/interrupt) the interrupt reader.
func (ir *InterruptReader) Start(ctx context.Context) (context.Context, context.CancelFunc) {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	if ir.cancel != nil {
		ir.cancel()
	}
	nctx, cancel := context.WithCancel(ctx)
	ir.cancel = cancel
	go func() {
		ir.start(nctx)
	}()
	return nctx, cancel
}

// Implement io.Reader interface.
func (ir *InterruptReader) Read(p []byte) (int, error) {
	ir.mu.Lock()
	for len(ir.buf) == 0 && ir.err == nil {
		ir.cond.Wait()
	}
	n := copy(p, ir.buf)
	if n == len(ir.buf) {
		ir.buf = ir.reset // consumed all, reset to initial buffer
	} else {
		ir.buf = ir.buf[n:] // partial read
	}
	err := ir.err
	ir.err = nil
	ir.mu.Unlock()
	return n, err
}

const CtrlC = 3 // Control-C is ascii 3 (C is 3rd letter of the alphabet)

func TimeoutToTimeval(timeout time.Duration) syscall.Timeval {
	return syscall.NsecToTimeval(timeout.Nanoseconds())
}

func TimeoutReader(fd int, tv syscall.Timeval, buf []byte) (int, error) {
	var readfds syscall.FdSet
	readfds.Bits[fd/64] |= (1 << (uint(fd) % 64))
	err := syscall.Select(fd+1, &readfds, nil, nil, &tv)
	if errors.Is(err, syscall.EINTR) {
		log.LogVf("Interrupted select")
		return 0, nil
	}
	if err != nil {
		log.Errf("Select error: %v", err)
		return 0, err
	}
	if readfds.Bits[fd/64]&(1<<(uint(fd)%64)) == 0 {
		return 0, nil // timeout case
	}
	n, err := syscall.Read(fd, buf)
	if n == 0 && err == nil {
		err = io.EOF
	}
	return n, err
}

func (ir *InterruptReader) start(ctx context.Context) {
	localBuf := make([]byte, ir.bufSize)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	// Check for signal and context every 250ms, though signals should interrupt the select,
	// they don't (at least on macOS, for the signals we are watching).
	tv := TimeoutToTimeval(250 * time.Millisecond)
	defer ir.cond.Signal()
	for {
		// log.Debugf("InterruptReader loop")
		select {
		case <-sigc:
			ir.setError(NewErrInterrupted("signal received"))
			ir.cancel()
			return
		case <-ctx.Done():
			ir.setError(NewErrInterruptedWithErr("context done", ctx.Err()))
			return
		default:
			n, err := TimeoutReader(ir.fd, tv, localBuf)
			if err != nil {
				ir.setError(err)
				return
			}
			if n == 0 {
				continue
			}
			localBuf = localBuf[:n]
			idx := bytes.IndexByte(localBuf, CtrlC)
			if idx != -1 {
				log.Infof("Ctrl-C found in input")
				localBuf = localBuf[:idx] // discard ^C and the rest.
				ir.mu.Lock()
				ir.cancel()
				ir.buf = append(ir.buf, localBuf...)
				ir.err = ErrUserInterrupt
				ir.mu.Unlock()
				return
			}
			ir.mu.Lock()
			ir.buf = append(ir.buf, localBuf...) // Might grow unbounded if not read.
			ir.cond.Signal()
			ir.mu.Unlock()
		}
	}
}

func (ir *InterruptReader) setError(err error) {
	log.Infof("InterruptReader setting error: %v", err)
	ir.mu.Lock()
	ir.err = err
	ir.mu.Unlock()
}

func SleepWithContext(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		// Completed the sleep duration
		return nil
	case <-ctx.Done():
		// Context was canceled
		return ctx.Err()
	}
}
