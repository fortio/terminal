//go:build windows || test_alt_timeoutreader

package terminal

import (
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"fortio.org/log"
	"fortio.org/safecast"
	"golang.org/x/sys/windows"
)

const IsUnix = false

type SystemTimeoutReader = TimeoutReaderWindows

func NewSystemTimeoutReader(stream *os.File, timeout time.Duration) *TimeoutReaderWindows {
	return NewTimeoutReaderWindows(stream, timeout)
}

type TimeoutReaderWindows struct {
	handle              syscall.Handle
	timeoutMilliseconds uint32
	blocking            bool
	ostream             *os.File
	buf                 []byte
	mut                 sync.Mutex
}

const fdwMode = windows.ENABLE_EXTENDED_FLAGS

func NewTimeoutReaderWindows(stream *os.File, timeout time.Duration) *TimeoutReaderWindows {
	if timeout < 0 {
		panic("Timeout must be greater than or equal to 0")
	}
	// if we don't set console mode, the inputrecord struct's values become.... corrupted?
	err := windows.SetConsoleMode(windows.Handle(stream.Fd()), uint32(fdwMode))
	if err != nil {
		// TODO: decide how to handle this
	}
	return &TimeoutReaderWindows{
		handle:              syscall.Handle(os.Stdin.Fd()),
		timeoutMilliseconds: safecast.MustConv[uint32](timeout.Milliseconds()),
		blocking:            timeout == 0,
		ostream:             stream,
	}
}

func (tr *TimeoutReaderWindows) IsClosed() bool {
	return tr.ostream == nil
}

func (tr *TimeoutReaderWindows) Close() error {
	var err error
	if tr.blocking && tr.ostream != nil {
		err = tr.ostream.Close()
		tr.ostream = nil
	}
	return err
}

func (tr *TimeoutReaderWindows) RawMode() error {
	return nil
}

func (tr *TimeoutReaderWindows) NormalMode() error {
	return nil
}

func (tr *TimeoutReaderWindows) StartDirect() {
}

func (tr *TimeoutReaderWindows) ChangeTimeout(timeout time.Duration) {
	tr.timeoutMilliseconds = safecast.MustConv[uint32](timeout.Milliseconds())
}

func (tr *TimeoutReaderWindows) ReadBlocking(p []byte) (int, error) {
	// TODO: figure out why we need these two lines every time we ReadBlocking but not for ReadWithTimeout
	fdwMode := windows.ENABLE_EXTENDED_FLAGS
	_ = windows.SetConsoleMode(windows.Handle(tr.handle), uint32(fdwMode))
	var iR InputRecord
	var read uint32
	err := ReadConsoleInput(tr.handle, &iR, 1, &read)
	if err != nil {
		log.Errf("ReadConsoleInput error: %v", err)
		return 0, err
	}
	nilCheck := InputRecord{}
	if iR == nilCheck {
		return 0, nil // timeout case
	}
	return iR.Read(p)
}

func (tr *TimeoutReaderWindows) PrimeReadImmediate(buf []byte) {
	tr.buf = buf
}

func (tr *TimeoutReaderWindows) Read(buf []byte) (int, error) {
	tr.mut.Lock()
	defer tr.mut.Unlock()
	if tr.blocking {
		return tr.ReadBlocking(buf)
	}
	return ReadWithTimeout(tr.handle, tr.timeoutMilliseconds, buf)
}

func (tr *TimeoutReaderWindows) ReadImmediate() (int, error) {
	if tr.blocking {
		return tr.ReadBlocking(tr.buf)
	}
	return ReadWithTimeout(tr.handle, 0, tr.buf)
}

func (tr *TimeoutReaderWindows) ReadWithTimeout(buf []byte) (int, error) {
	return ReadWithTimeout(tr.handle, tr.timeoutMilliseconds, buf)
}

const (
	WAITOBJECT0 = 0x00000000
	WAITTIMEOUT = 0x00000102
)

func ReadWithTimeout(handle syscall.Handle, ms uint32, buf []byte) (int, error) {
	var iR InputRecord
	var read uint32
	event, err := windows.WaitForSingleObject(windows.Handle(handle), ms)
	if event == WAITTIMEOUT {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	err = ReadConsoleInput(handle, &iR, 1, &read)
	if err != nil {
		log.Errf("ReadConsoleInput error: %v", err)
		return 0, err
	}
	nilCheck := InputRecord{}
	if iR == nilCheck {
		return 0, nil // timeout case
	}
	n, err := iR.Read(buf)

	return n, err
}

var (
	procReadConsoleInputW = modkernel32.NewProc("ReadConsoleInputW")
	modkernel32           = windows.NewLazySystemDLL("kernel32.dll")
)

func ReadConsoleInput(console syscall.Handle, rec *InputRecord, toread uint32, read *uint32) (err error) {
	r1, _, e1 := syscall.Syscall6(procReadConsoleInputW.Addr(), 4,
		uintptr(console), uintptr(unsafe.Pointer(rec)), uintptr(toread),
		uintptr(unsafe.Pointer(read)), 0, 0)
	if r1 == 0 {
		err = errnoErr(e1)
	}
	return
}

type InputRecord struct {
	// 0x1: Key event
	// 0x2: Will never be read when using ReadConsoleInput
	// 0x4: Window buffer size event
	// 0x8: Deprecated
	// 0x10: Deprecated
	// Original source: https://docs.microsoft.com/en-us/windows/console/input-record-str#members
	Type uint16

	// _ [2]uint16
	// Data contents are:
	// If the event is a key event (Type == 1):
	//  - Data[0] is 0x1 if the key is pressed, 0x0 if the key is released
	//  - Data[3] is the keycode of the pressed key, see
	//    https://docs.microsoft.com/en-us/windows/win32/inputdev/virtual-key-codes
	//  - Data[5] is the ascii or Unicode keycode.
	//  - Data[6] stores the state of the modifier keys.
	//  Original source: https://docs.microsoft.com/en-us/windows/console/key-event-record-str
	//  If the event is a mouse event

	//
	// If the event is a window buffer size event (Type == 4):
	//  - Data[0] is the new amount of character rows
	//  - Data[1] is the new amount of character columns
	// Original source: https://docs.microsoft.com/en-us/windows/console/window-buffer-size-record-str
	Data [8]uint16
}

func (ir *InputRecord) Read(buf []byte) (int, error) {
	// TODO: fully create function to translate keypresses to buffer
	switch ir.Type {
	case 0x1: // key event
		if ir.Data[1] == 0 {
			return 0, nil
		}
		arrowCheck1, arrowCheck2 := ir.Data[4], ir.Data[5]
		switch {
		case arrowCheck1 == 38 && arrowCheck2 == 72: // up
			copy(buf, "\x1b[A")
			return 3, nil
		case arrowCheck1 == 40 && arrowCheck2 == 80: // down
			copy(buf, "\x1b[B")
			return 3, nil
		case arrowCheck1 == 37 && arrowCheck2 == 75: // left
			copy(buf, "\x1b[D")
			return 3, nil
		case arrowCheck1 == 39 && arrowCheck2 == 77: // right
			copy(buf, "\x1b[C")
			return 3, nil
		}
		copy(buf, []byte{byte(ir.Data[6])})
		// buf[0] = byte(ir.Data[6])
		return 1, nil
	case 0x4: // window buffer size event
		// TODO: decide best approach for handling window size events
	}
	return 0, nil
}

func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return errERROREINVAL
	case errnoERRORIOPENDING:
		return errERRORIOPENDING
	default:
		return e
	}
}

// Do the interface allocations only once for common
// Errno values.
const (
	errnoERRORIOPENDING = 997
)

var (
	errERRORIOPENDING error = syscall.Errno(errnoERRORIOPENDING)
	errERROREINVAL    error = syscall.EINVAL
)
