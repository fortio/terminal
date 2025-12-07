//go:build windows || test_alt_timeoutreader

package terminal

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"fortio.org/log"
	"golang.org/x/sys/windows"
)

const IsUnix = false

type SystemTimeoutReader = TimeoutReaderWindows

func NewSystemTimeoutReader(stream *os.File, timeout time.Duration) *TimeoutReaderWindows {
	return NewTimeoutReaderWindows(stream, timeout)
}

type TimeoutReaderWindows struct {
	handle   syscall.Handle
	timeout  time.Duration
	blocking bool
	ostream  *os.File
	buf      []byte
	mut      sync.Mutex
}

func NewTimeoutReaderWindows(stream *os.File, timeout time.Duration) *TimeoutReaderWindows {
	if timeout < 0 {
		panic("Timeout must be greater than or equal to 0")
	}
	return &TimeoutReaderWindows{
		handle:   syscall.Handle(os.Stdin.Fd()),
		timeout:  timeout,
		blocking: timeout == 0,
		ostream:  stream,
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
	tr.timeout = timeout
}

func (tr *TimeoutReaderWindows) ReadBlocking(p []byte) (int, error) {
	var iR InputRecord
	var read uint32
	err := ReadConsoleInput(syscall.Handle(tr.handle), &iR, 1, &read)
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
	return ReadWithTimeout(tr.handle, tr.timeout, buf)
}

func (tr *TimeoutReaderWindows) ReadImmediate() (int, error) {
	if tr.blocking {
		return tr.ReadBlocking(tr.buf)
	}
	return ReadWithTimeout(tr.handle, time.Duration(0), tr.buf)
}

func (tr *TimeoutReaderWindows) ReadWithTimeout(buf []byte) (int, error) {
	return ReadWithTimeout(tr.handle, tr.timeout, buf)
}

const (
	WAIT_OBJECT_0 = 0x00000000
	WAIT_TIMEOUT  = 0x00000102
)

func ReadWithTimeout(handle syscall.Handle, tv time.Duration, buf []byte) (int, error) {
	var iR InputRecord
	var read uint32

	event, err := windows.WaitForSingleObject(windows.Handle(handle), uint32(tv))
	if event == WAIT_TIMEOUT {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	err = ReadConsoleInput(syscall.Handle(handle), &iR, 1, &read)
	fmt.Println(iR.Data)
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

// TODO: fully create function to go
func (ir *InputRecord) Read(buf []byte) (int, error) {
	log.Infof("falkdjfas")
	fmt.Println(ir.Data)
	switch ir.Type {
	case 0x1: // key event
		if ir.Data[1] == 0 {
			return 0, nil
		}
		arrowCheck1, arrowCheck2 := ir.Data[4], ir.Data[5]
		fmt.Println(arrowCheck1, arrowCheck2)
		switch {
		case arrowCheck1 == 38 && arrowCheck2 == 72: // up
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

	}
	return 0, nil
}

func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return errERROR_EINVAL
	case errnoERROR_IO_PENDING:
		return errERROR_IO_PENDING
	}
	return e
}

// Do the interface allocations only once for common
// Errno values.
const (
	errnoERROR_IO_PENDING = 997
)

var (
	errERROR_IO_PENDING error = syscall.Errno(errnoERROR_IO_PENDING)
	errERROR_EINVAL     error = syscall.EINVAL
)
