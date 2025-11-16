package terminal

import (
	"errors"
	"io"
	"sync"
	"time"

	"fortio.org/log"
)

var ErrDataTruncated = errors.New("data truncated")

// readResult holds the outcome of a read operation performed by the background goroutine.
type readResult struct {
	n    int    // number of bytes read
	data []byte // the data, buffer returned back from the first/original Read()
	err  error
}

// TimeoutReader wraps an io.Reader (typically os.Stdin) to provide read operations
// with a timeout using a persistent background reader goroutine and internal buffering.
// It also allows a reset/restart without loosing data to a leftover/pending read if
// the reset is triggered by say reading a ^C from the input (which will unblock the read).
type TimeoutReader struct {
	file    io.Reader
	timeout time.Duration

	inRead     bool            // Indicates if a read is in progress/hasn't returned yet
	inputChan  chan []byte     // Read() -> goroutine buffer passing and signaling that it's ok to read
	resultChan chan readResult // goroutine back to Read() result passing.
	stopChan   chan struct{}   // Signal channel to stop the goroutine early on e.g. Close() (maybe inputChan is enough?)
	wg         sync.WaitGroup  // To wait for the goroutine to exit
	mu         sync.Mutex      // Protects inRead and lastErr
	lastErr    error           // Stores persistent errors like EOF - TODO: probably not needed
	closed     bool            // To signal if Close() has been called
	blocking   bool            // true if the reader is blocking (timeout == 0), false if it has a timeout set
}

// NewTimeoutReader creates a new TimeoutReader with a persistent background reader.
// The timeout applies to each Read call waiting for new data.
// A duration of 0 or less is invalid and will panic.
func NewTimeoutReader(stream io.Reader, timeout time.Duration) *TimeoutReader {
	log.LogVf("Creating non select based TimeoutReader with timeout: %v", timeout)
	if timeout < 0 {
		panic("Timeout must be greater or equal to 0")
	}
	blocking := (timeout == 0)
	tr := &TimeoutReader{
		file:     stream,
		timeout:  timeout,
		blocking: blocking,
	}

	if !blocking {
		tr.inputChan = make(chan []byte, 1)
		tr.resultChan = make(chan readResult)
		tr.stopChan = make(chan struct{})
		tr.wg.Add(1)
		go tr.readerLoop()
	}

	return tr
}

// readerLoop is the background goroutine that continuously reads from the file.
func (tr *TimeoutReader) readerLoop() {
	defer tr.wg.Done()
	defer close(tr.resultChan) // Close dataChan when loop exits
	for {
		// Wait for the signal to read
		log.Debugf("Waiting for ok to read")
		readBuf, ok := <-tr.inputChan
		if !ok {
			log.LogVf("Exiting readloop from input channel closed")
			return // Input channel closed, exit the loop.
		}
		log.Debugf("Before reading (up to %d bytes)", len(readBuf))
		n, err := tr.file.Read(readBuf)
		log.Debugf("Done reading %d %v", n, err)
		result := readResult{n: n, data: readBuf, err: err}
		select {
		case tr.resultChan <- result:
			if err != nil {
				log.LogVf("Exiting readloop from error %v", err)
				return // Error or EOF occurred, stop reading.
			}
		case <-tr.stopChan:
			log.LogVf("Exiting readloop from stop channel")
			return // Stop signal received.
		}
	}
}

// Read attempts to read into the buffer buf.
// It first reads from an internal buffer. If the buffer is empty,
// it waits up to the configured timeout for new data from the background reader.
// Returns the number of bytes read and an error. If a timeout occurs while
// waiting for new data, it returns (0, nil), indicating no data read and no error.
// Note: You should not call Read() with a smaller buffer than the first one when it returns
// early due to timeout as to avoid extra allocations the inflight buffer will be used
// and thus could have more data than the new buffer, it will be lost/truncated in that case
// and the error ErrDataTruncated will be returned.
func (tr *TimeoutReader) Read(buf []byte) (int, error) {
	if tr.blocking {
		return tr.file.Read(buf)
	}
	sameBuf := false
	tr.mu.Lock()
	defer tr.mu.Unlock()
	// If we are already in a read, we don't want to send to the inputChan, we'll reuse the one in flight.
	if !tr.inRead {
		log.Debugf("Normal: Not in read, sending to inputChan")
		tr.inputChan <- buf // Send what to read and signal to the goroutine to do read
		sameBuf = true
		tr.inRead = true
	}
	timer := time.NewTimer(tr.timeout)
	defer timer.Stop()
	select {
	case res, ok := <-tr.resultChan:
		if !ok {
			// The reader loop has exited, no more data will be sent.
			return 0, tr.lastErr
		}
		tr.inRead = false
		if res.err != nil {
			tr.lastErr = res.err
		}
		if sameBuf {
			return res.n, res.err
		}
		if res.n > len(buf) {
			// Unexpected.
			log.Warnf("Read %d bytes from earlier Read request, but new buffer is only %d bytes", res.n, len(buf))
			res.err = ErrDataTruncated
		}
		n := copy(buf, res.data[:res.n]) // Copy the data to the provided buffer
		return n, res.err
	case <-timer.C:
		// Timeout occurred waiting for data from the background goroutine.
		// Return 0 bytes read and nil error, mimicking select() timeout behavior.
		return 0, nil
	}
}

// ReadBlocking is like Read but makes sure something (or an error) is returned/read.
func (tr *TimeoutReader) ReadBlocking(buf []byte) (int, error) {
	if tr.blocking {
		return tr.file.Read(buf)
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	// If we are already in a read, we don't want to send to the inputChan, we'll reuse the one in flight.
	if !tr.inRead {
		log.Debugf("Blocking: Not in read, direct read for ReadBlocking")
		return tr.file.Read(buf) // Direct read if not already in a read.
	}
	res, ok := <-tr.resultChan
	if !ok {
		// The reader loop has exited, no more data will be sent.
		return 0, tr.lastErr
	}
	tr.inRead = false
	if res.err != nil {
		tr.lastErr = res.err
	}
	if res.n > len(buf) {
		// Unexpected.
		log.Warnf("Read %d bytes from earlier Read request, but new buffer is only %d bytes", res.n, len(buf))
		res.err = ErrDataTruncated
	}
	n := copy(buf, res.data[:res.n]) // Copy the data to the provided buffer
	return n, res.err
}

// ReadImmediate attempts to read into the buffer buf if there is something immediately available.
func (tr *TimeoutReader) ReadImmediate(buf []byte) (int, error) {
	if tr.blocking {
		panic("ReadImmediate not meaningful in blocking mode")
	}
	sameBuf := false
	tr.mu.Lock()
	defer tr.mu.Unlock()
	// If we are already in a read, we don't want to send to the inputChan, we'll reuse the one in flight.
	if !tr.inRead {
		log.Debugf("Immediate: Not in read, sending to inputChan")
		tr.inputChan <- buf // Send what to read and signal to the goroutine to do read
		sameBuf = true
		tr.inRead = true
	}
	select {
	case res, ok := <-tr.resultChan:
		if !ok {
			// The reader loop has exited, no more data will be sent.
			return 0, tr.lastErr
		}
		tr.inRead = false
		if res.err != nil {
			tr.lastErr = res.err
		}
		if sameBuf {
			return res.n, res.err
		}
		if res.n > len(buf) {
			// Unexpected.
			log.Warnf("Read %d bytes from earlier Read request, but new buffer is only %d bytes", res.n, len(buf))
			res.err = ErrDataTruncated
		}
		n := copy(buf, res.data[:res.n]) // Copy the data to the provided buffer
		return n, res.err
	default:
		// no data ready yet (will be in next call most likely)
		return 0, nil
	}
}

// ReadWithTimeout is an alias to [TimeoutReader.Read] for the InputReader interface compatibility.
func (tr *TimeoutReader) ReadWithTimeout(buf []byte) (int, error) {
	return tr.Read(buf)
}

// ChangeTimeout updates the timeout duration for subsequent Read calls
// when waiting for new data from the background reader. If called currently
// it will block until the current read completes.
func (tr *TimeoutReader) ChangeTimeout(newTimeout time.Duration) {
	log.LogVf("Changing non select based TimeoutReader to timeout: %v", newTimeout)
	if tr.blocking && newTimeout != 0 {
		panic("Cannot change from blocking to non-blocking mode")
	}
	if newTimeout <= 0 {
		panic("Timeout must be greater than zero unless started and staying in blocking mode")
	}
	tr.mu.Lock()
	tr.timeout = newTimeout
	tr.mu.Unlock()
}

// Close signals the background reader goroutine to stop and waits for it to exit.
// It purposely doesn't close the underlying file unless in blocking mode and it implements io.Closer.
func (tr *TimeoutReader) Close() error {
	if tr.blocking && tr.file != nil {
		var err error
		if closer, ok := tr.file.(io.Closer); ok {
			err = closer.Close()
		}
		tr.file = nil // Clear the stream reference
		return err
	}
	log.LogVf("Closing non select based TimeoutReader")
	tr.mu.Lock()
	tr.closed = true
	select {
	case <-tr.stopChan:
		tr.mu.Unlock()
		return nil // Already closed
	default:
	}
	log.LogVf("Closing stop and input channel")
	close(tr.stopChan)
	close(tr.inputChan)
	tr.mu.Unlock()
	tr.wg.Wait()
	return nil
}

// IsClosed returns true if Close() has been called (and for the other implementation a new one should be created).
func (tr *TimeoutReader) IsClosed() bool {
	if tr.blocking {
		return tr.file == nil
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.closed
}
