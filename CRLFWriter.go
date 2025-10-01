package terminal

import (
	"bytes"
	"io"
	"strings"
	"sync"
)

type CRLFWriter struct {
	// Out is the underlying writer to write to.
	Out io.Writer
}

func (w *CRLFWriter) Write(buf []byte) (n int, err error) {
	return CRLFWrite(w.Out, buf)
}

func CRLFWrite(out io.Writer, buf []byte) (n int, err error) {
	// Somewhat copied from x/term's writeWithCRLF
	for len(buf) > 0 {
		i := bytes.IndexByte(buf, '\n')
		todo := len(buf)
		if i >= 0 {
			todo = i
		}
		var nn int
		nn, err = out.Write(buf[:todo])
		n += nn
		if err != nil {
			return n, err
		}
		buf = buf[todo:]
		if i >= 0 {
			if _, err = out.Write([]byte{'\r', '\n'}); err != nil {
				return n, err
			}
			n++
			buf = buf[1:]
		}
	}
	// Auto flush
	if flusher, ok := out.(FlushWriter); ok {
		err = flusher.Flush()
	}
	return n, err
}

func (w *CRLFWriter) Flush() error {
	// flush already done at the end of Write.
	return nil
}

type FlushWriter interface {
	io.Writer
	Flush() error
}

type Bufio interface {
	FlushWriter
	io.StringWriter
	io.ByteWriter
	WriteRune(r rune) (n int, err error)
}

// SyncWriter is a threadsafe wrapper around a most of the APIs of bufio.Writer.
type SyncWriter struct {
	// Out is the underlying writer to write to.
	Out Bufio
	// mu protects access to the Out writer.
	mu sync.Mutex
}

func (w *SyncWriter) Write(buf []byte) (n int, err error) {
	w.mu.Lock()
	n, err = w.Out.Write(buf)
	w.mu.Unlock()
	return n, err
}

func (w *SyncWriter) Flush() error {
	w.mu.Lock()
	err := w.Out.Flush()
	w.mu.Unlock()
	return err
}

func (w *SyncWriter) WriteString(s string) (n int, err error) {
	w.mu.Lock()
	n, err = w.Out.WriteString(s)
	w.mu.Unlock()
	return n, err
}

func (w *SyncWriter) WriteByte(c byte) error {
	w.mu.Lock()
	err := w.Out.WriteByte(c)
	w.mu.Unlock()
	return err
}

func (w *SyncWriter) WriteRune(r rune) (n int, err error) {
	w.mu.Lock()
	n, err = w.Out.WriteRune(r)
	w.mu.Unlock()
	return n, err
}

// Lock: Shares the underlying lock.
func (w *SyncWriter) Lock() {
	w.mu.Lock()
}

// Unlock: Shares the underlying lock.
func (w *SyncWriter) Unlock() {
	w.mu.Unlock()
}

type FlushableStringBuilder struct {
	strings.Builder
}

func (b *FlushableStringBuilder) Flush() error {
	return nil
}

type FlushableBytesBuffer struct {
	bytes.Buffer
}

func (b *FlushableBytesBuffer) Flush() error {
	return nil
}
