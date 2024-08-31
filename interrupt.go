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
	buf     []byte
	bufSize int
	err     error
	mu      sync.Mutex
	cancel  context.CancelFunc
}

var ErrInterrupted = errors.New("terminal interrupted")

// NewInterruptReader creates a new interrupt reader.
// it needs to be Start()ed to start reading from the underlying reader
// and intercept Ctrl-C and listen for interrupt signals.
func NewInterruptReader(reader io.Reader, bufSize int) *InterruptReader {
	ir := &InterruptReader{
		reader:  reader,
		bufSize: bufSize,
		buf:     make([]byte, 0, bufSize),
	}
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
	n := copy(p, ir.buf)
	ir.buf = ir.buf[n:]
	err := ir.err
	ir.err = nil
	ir.mu.Unlock()
	return n, err
}

const CtrlC = 3

func (ir *InterruptReader) start(ctx context.Context) {
	localBuf := make([]byte, ir.bufSize)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sigc:
			log.Infof("Interrupted by signal")
			ir.cancel()
			ir.mu.Lock()
			ir.err = ErrInterrupted
			ir.mu.Unlock()
			return
		case <-ctx.Done():
			log.Infof("Context done")
			ir.mu.Lock()
			ir.err = ErrInterrupted
			ir.mu.Unlock()
			return
		default:
			n, err := ir.reader.Read(localBuf)
			if err != nil {
				ir.mu.Lock()
				ir.err = err
				ir.mu.Unlock()
				return
			}
			localBuf = localBuf[:n]
			idx := bytes.IndexByte(localBuf, CtrlC)
			if idx != -1 {
				log.Infof("Ctrl-C found in input")
				localBuf = localBuf[:idx] // discard ^c and the rest.
				ir.cancel()
			}
			ir.mu.Lock()
			ir.buf = append(ir.buf, localBuf...) // Might grow unbounded if not read.
			ir.mu.Unlock()
		}
	}
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
