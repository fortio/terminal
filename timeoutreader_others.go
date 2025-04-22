//go:build !unix
// +build !unix

package terminal

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

const IsUnix = false

// readResult holds the outcome of a read operation performed by the background goroutine.
type readResult struct {
	data []byte // A copy of the data read
	err  error
}

// TimeoutReader wraps an os.File (typically os.Stdin) to provide read operations
// with a timeout using a persistent background reader goroutine and internal buffering.
type TimeoutReader struct {
	file    *os.File
	timeout time.Duration

	internalBuf bytes.Buffer // Buffer for data read ahead
	dataChan    chan readResult
	stopChan    chan struct{}  // Signal channel to stop the goroutine
	wg          sync.WaitGroup // To wait for the goroutine to exit
	mu          sync.Mutex     // Protects internalBuf and lastErr
	lastErr     error          // Stores persistent errors like EOF
}

// NewTimeoutReader creates a new TimeoutReader with a persistent background reader.
// The timeout applies to each Read call waiting for new data.
// A duration of 0 or less disables the timeout for waiting on new data.
func NewTimeoutReader(stream *os.File, timeout time.Duration) *TimeoutReader {
	tr := &TimeoutReader{
		file:     stream,
		timeout:  timeout,
		dataChan: make(chan readResult, 1),
		stopChan: make(chan struct{}),
	}

	tr.wg.Add(1)
	go tr.readerLoop()

	return tr
}

// readerLoop is the background goroutine that continuously reads from the file.
func (tr *TimeoutReader) readerLoop() {
	defer tr.wg.Done()
	defer close(tr.dataChan) // Close dataChan when loop exits

	readBuf := make([]byte, 4096) // Adjust size as needed

	for {
		n, err := tr.file.Read(readBuf)
		dataCopy := make([]byte, n)
		copy(dataCopy, readBuf[:n])
		result := readResult{data: dataCopy, err: err}

		select {
		case tr.dataChan <- result:
			if err != nil {
				return // Error or EOF occurred, stop reading.
			}
		case <-tr.stopChan:
			return // Stop signal received.
		}
	}
}

// Read attempts to read into the buffer buf.
// It first reads from an internal buffer. If the buffer is empty,
// it waits up to the configured timeout for new data from the background reader.
// Returns the number of bytes read and an error. If a timeout occurs while
// waiting for new data, it returns (0, nil), indicating no data read and no error.
func (tr *TimeoutReader) Read(buf []byte) (int, error) {
	tr.mu.Lock()
	// 1. Try reading from the internal buffer first.
	if tr.internalBuf.Len() > 0 {
		n, _ := tr.internalBuf.Read(buf)
		// Check if EOF should be reported now
		errToReturn := tr.lastErr
		if tr.internalBuf.Len() > 0 {
			errToReturn = nil // Don't return EOF if buffer still has data
		}
		tr.mu.Unlock()
		return n, errToReturn
	}

	// 2. Check if we already encountered a persistent error (like EOF).
	if tr.lastErr != nil {
		errToReturn := tr.lastErr
		tr.mu.Unlock()
		return 0, errToReturn
	}
	tr.mu.Unlock() // Unlock before potentially blocking

	// 3. Internal buffer is empty, wait for data or timeout.
	timer := time.NewTimer(tr.timeout)
	defer timer.Stop()
	select {
	case res, ok := <-tr.dataChan:
		tr.mu.Lock()
		if !ok {
			// Channel closed by the goroutine.
			if tr.lastErr == nil {
				tr.lastErr = io.EOF
			}
			errToReturn := tr.lastErr
			tr.mu.Unlock()
			return 0, errToReturn
		}

		// Received new data or error
		isEOF := errors.Is(res.err, io.EOF)
		if res.err != nil && !isEOF {
			tr.lastErr = res.err
			errToReturn := tr.lastErr
			tr.mu.Unlock()
			return 0, errToReturn
		}

		if len(res.data) > 0 {
			tr.internalBuf.Write(res.data)
		}

		if isEOF {
			tr.lastErr = io.EOF
		}

		// 4. Read from the potentially populated internal buffer
		n, _ := tr.internalBuf.Read(buf)
		errToReturn := tr.lastErr
		if tr.internalBuf.Len() > 0 {
			errToReturn = nil // Don't return EOF if buffer still has data
		}
		tr.mu.Unlock()
		return n, errToReturn

	case <-timer.C:
		// Timeout occurred waiting for data from the background goroutine.
		// Return 0 bytes read and nil error, mimicking select() timeout behavior.
		return 0, nil
	}
}

// ChangeTimeout updates the timeout duration for subsequent Read calls
// when waiting for new data from the background reader.
func (tr *TimeoutReader) ChangeTimeout(newTimeout time.Duration) {
	tr.mu.Lock()
	tr.timeout = newTimeout
	tr.mu.Unlock()
}

// Close signals the background reader goroutine to stop and waits for it to exit.
func (tr *TimeoutReader) Close() error {
	tr.mu.Lock()
	select {
	case <-tr.stopChan:
		tr.mu.Unlock()
		return nil // Already closed
	default:
	}
	close(tr.stopChan)
	tr.mu.Unlock()
	tr.wg.Wait()
	return nil
}
