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
	"golang.org/x/term"
)

type InterruptReader struct {
	In      *os.File // stdin typically
	buf     []byte
	reset   []byte // original buffer start
	bufSize int
	err     error
	mu      sync.Mutex
	cond    sync.Cond
	cancel  context.CancelFunc
	timeout time.Duration
	stopped bool
	// TimeoutReader is the timeout reader for the interrupt reader. Do not access directly if also using Start/Stop.
	TR *TimeoutReader
	// Terminal state (raw mode vs normal)
	st *term.State
	// IgnoreRaw is set to true if the terminal doesn't support MakeRaw / ioctl,
	// but the caller wants to force it to be used anyway.
	IgnoreRaw bool
}

var (
	ErrUserInterrupt = NewErrInterrupted("terminal interrupted by user")
	ErrStopped       = NewErrInterrupted("interrupt reader stopped") // not really an error more of a marker.
	ErrSignal        = NewErrInterrupted("signal received")
)

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
// Use GetSharedInput() to get a shared interrupt reader across libraries/caller.
func NewInterruptReader(reader *os.File, bufSize int, timeout time.Duration) *InterruptReader {
	ir := &InterruptReader{
		In:      reader,
		bufSize: bufSize,
		timeout: timeout,
		buf:     make([]byte, 0, bufSize),
		TR:      NewTimeoutReader(reader, timeout),
	}
	ir.reset = ir.buf
	ir.cond = *sync.NewCond(&ir.mu)
	log.Config.GoroutineID = true
	return ir
}

func (ir *InterruptReader) ChangeTimeout(timeout time.Duration) {
	ir.mu.Lock()
	ir.timeout = timeout
	if ir.TR.IsClosed() {
		ir.TR = NewTimeoutReader(ir.In, timeout)
	} else {
		ir.TR.ChangeTimeout(timeout)
	}
	ir.mu.Unlock()
}

func (ir *InterruptReader) Stop() {
	log.Debugf("InterruptReader stopping")
	ir.mu.Lock()
	if ir.cancel == nil {
		ir.mu.Unlock()
		return
	}
	ir.cancel()
	ir.stopped = true
	ir.cancel = nil
	ir.mu.Unlock()
	_, _ = ir.Read([]byte{}) // wait for cancel.
	log.Debugf("InterruptReader done stopping")
	ir.mu.Lock()
	ir.buf = ir.reset
	ir.err = nil
	ir.mu.Unlock()
}

func (ir *InterruptReader) InEOF() bool {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	return errors.Is(ir.err, io.EOF)
}

// Start or restart (after a cancel/interrupt) the interrupt reader.
func (ir *InterruptReader) Start(ctx context.Context) (context.Context, context.CancelFunc) {
	log.Debugf("InterruptReader starting")
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.stopped = false
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
	n, err := ir.read(p)
	ir.mu.Unlock()
	return n, err
}

// ReadNonBlocking will read what is available already or return 0, nil if nothing is available.
func (ir *InterruptReader) ReadNonBlocking(p []byte) (int, error) {
	ir.mu.Lock()
	n, err := ir.read(p)
	ir.mu.Unlock()
	return n, err
}

// ReadLine reads until \r or \n (for use when not in rawmode).
// It returns the line (without the \r, \n, or \r\n).
func (ir *InterruptReader) ReadLine() (string, error) {
	needAtLeast := 0
	ir.mu.Lock()
	defer ir.mu.Unlock()
	for {
		// log.Debugf("ReadLine before loop for input %d", needAtLeast)
		for len(ir.buf) <= needAtLeast && ir.err == nil {
			// log.Debugf("ReadLine waiting for input %d", needAtLeast)
			ir.cond.Wait()
		}
		// log.Debugf("ReadLine after loop for input %d, %v", len(ir.buf), ir.err)
		err := ir.err
		line := ""
		for i, c := range ir.buf {
			switch c {
			case '\r':
				line = string(ir.buf[:i])
				// is there one more character and is it \n?
				if i < len(ir.buf)-1 && ir.buf[i+1] == '\n' {
					i++
				}
				fallthrough
			case '\n':
				if line == "" { // not fallthrough from \r
					line = string(ir.buf[:i])
				}
				ir.buf = ir.buf[i+1:]
				if len(ir.buf) == 0 {
					ir.buf = ir.reset
				}
				return line, nil
			}
		}
		needAtLeast = len(ir.buf)
		eof := false
		if errors.Is(err, io.EOF) && needAtLeast > 0 {
			// keep eof for next readline, first return the buffer, without the EOF
			eof = true
			err = nil
		}
		if err != nil || eof {
			line = string(ir.buf)
			ir.buf = ir.reset
			return line, err
		}
	}
}

func (ir *InterruptReader) read(p []byte) (int, error) {
	n := copy(p, ir.buf)
	if n == len(ir.buf) {
		ir.buf = ir.reset // consumed all, reset to initial buffer
	} else {
		ir.buf = ir.buf[n:] // partial read
	}
	err := ir.err
	if !errors.Is(err, io.EOF) { // EOF is sticky.
		ir.err = nil
	}
	return n, err
}

const CtrlC = 3 // Control-C is ascii 3 (C is 3rd letter of the alphabet)

func (ir *InterruptReader) start(ctx context.Context) {
	localBuf := make([]byte, ir.bufSize)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	// Check for signal and context every ir.timeout, though signals should interrupt the select,
	// they don't (at least on macOS, for the signals we are watching).
	tr := ir.TR
	if tr.IsClosed() {
		tr = NewTimeoutReader(ir.In, ir.timeout)
		ir.TR = tr
	} else {
		tr.ChangeTimeout(ir.timeout)
	}
	defer tr.Close()
	defer ir.cond.Signal()
	for {
		// log.Debugf("InterruptReader loop")
		select {
		case <-sigc:
			ir.setError(ErrSignal)
			ir.cancel()
			return
		case <-ctx.Done():
			ir.mu.Lock()
			stopped := ir.stopped
			ir.mu.Unlock()
			if stopped {
				ir.setError(ErrStopped)
				ir.cond.Broadcast()
			} else {
				ir.setError(NewErrInterruptedWithErr("context done", ctx.Err()))
			}
			return
		default:
			n, err := tr.Read(localBuf)
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
			delay := false
			if localBuf[n-1] == '\r' || localBuf[n-1] == '\n' {
				// We just ended on a new line (\r in raw mode). We will want to wait before the next read.
				delay = true
			}
			ir.mu.Lock()
			ir.buf = append(ir.buf, localBuf...) // Might grow unbounded if not read.
			ir.cond.Signal()
			ir.mu.Unlock()
			if delay {
				// This is a bit of a hack to give a chance to caller of ReadLine
				// to stop the goroutine based timeout_reader before it enters the next read.
				_ = SleepWithContext(ctx, ir.timeout/5)
			}
		}
	}
}

func (ir *InterruptReader) setError(err error) {
	level := log.Info
	if errors.Is(err, ErrStopped) {
		level = log.Verbose
	}
	log.S(level, "InterruptReader setting error", log.Any("err", err))
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
