//go:build windows && !test_alt_timeoutreader

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

const (
	ResizeSignal = syscall.Signal(999)
)

type SystemTimeoutReader = TimeoutReaderWindows

func NewSystemTimeoutReader(stream *os.File, timeout time.Duration, signalChan chan os.Signal) *TimeoutReaderWindows {
	return NewTimeoutReaderWindows(stream, timeout, signalChan)
}

type TimeoutReaderWindows struct {
	handle              windows.Handle
	timeoutMilliseconds uint32
	blocking            bool
	inRead              bool
	ostream             *os.File
	signalChannel       chan os.Signal
	buf                 []byte
	mut                 sync.Mutex
}

const FDWMODE = windows.ENABLE_EXTENDED_FLAGS |
	windows.ENABLE_WINDOW_INPUT |
	windows.ENABLE_MOUSE_INPUT &
		^windows.ENABLE_QUICK_EDIT_MODE

func NewTimeoutReaderWindows(stream *os.File, timeout time.Duration, signalChan chan os.Signal) *TimeoutReaderWindows {
	if timeout < 0 {
		panic("Timeout must be greater than or equal to 0")
	}
	return &TimeoutReaderWindows{
		handle:              windows.Handle(syscall.Handle(stream.Fd())),
		timeoutMilliseconds: safecast.MustConv[uint32](timeout.Milliseconds()),
		blocking:            timeout == 0,
		ostream:             stream,
		signalChannel:       signalChan,
		inRead:              false,
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

func (tr *TimeoutReaderWindows) ChangeTimeout(timeout time.Duration) {
	tr.timeoutMilliseconds = safecast.MustConv[uint32](timeout.Milliseconds())
}

func (tr *TimeoutReaderWindows) ReadBlocking(p []byte) (int, error) {
	var curMode uint32
	err := windows.GetConsoleMode(tr.handle, &curMode)
	if err != nil {
		// TODO: handle error
		return 0, err
	}
	if curMode != FDWMODE {
		err = windows.SetConsoleMode(tr.handle, uint32(FDWMODE))
		if err != nil {
			return 0, err
		}
	}

	var iR InputRecords
	var read uint32
	err = ReadConsoleInput(tr.handle, &iR, 8, &read)
	if err != nil {
		log.Errf("ReadConsoleInput error: %v", err)
		return 0, err
	}
	return iR.Translate(p, tr.signalChannel, int(read))
}

func (tr *TimeoutReaderWindows) PrimeReadImmediate(buf []byte) {
	tr.buf = buf
}

func (tr *TimeoutReaderWindows) Read(buf []byte) (int, error) {
	if tr.blocking {
		return tr.ReadBlocking(buf)
	}
	tr.mut.Lock()
	defer tr.mut.Unlock()
	return ReadWithTimeout(tr.handle, tr.timeoutMilliseconds, buf, tr.signalChannel)
}

func (tr *TimeoutReaderWindows) ReadImmediate() (int, error) {
	if tr.blocking {
		return tr.ReadBlocking(tr.buf)
	}
	return ReadWithTimeout(tr.handle, 0, tr.buf, tr.signalChannel)
}

func (tr *TimeoutReaderWindows) ReadWithTimeout(buf []byte) (int, error) {
	return ReadWithTimeout(tr.handle, tr.timeoutMilliseconds, buf, tr.signalChannel)
}

const (
	WAITOBJECT0 = 0x00000000
	WAITTIMEOUT = 0x00000102
)

func ReadWithTimeout(handle windows.Handle, ms uint32, buf []byte, signalChan chan os.Signal) (int, error) {
	var iR InputRecords
	var read uint32
	event, err := windows.WaitForSingleObject(handle, ms)
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
	return iR.Translate(buf, signalChan, int(read))
}

var (
	procReadConsoleInputW = modkernel32.NewProc("ReadConsoleInputW")
	modkernel32           = windows.NewLazySystemDLL("kernel32.dll")
)

func ReadConsoleInput(console windows.Handle, rec *InputRecords, toread uint32, read *uint32) error {
	// r1, _, e1 := syscall.SyscallN(procReadConsoleInputW.Addr(), 4,
	// 	uintptr(console), uintptr(unsafe.Pointer(rec)), uintptr(toread),
	// 	uintptr(unsafe.Pointer(read)), 0, 0)
	r1, _, e1 := syscall.Syscall6(procReadConsoleInputW.Addr(), 4,
		uintptr(console), uintptr(unsafe.Pointer(rec)), uintptr(toread),
		uintptr(unsafe.Pointer(read)), 0, 0)
	var err error
	if r1 == 0 {
		err = errnoErr(e1)
	}
	return err
}

type InputRecord struct {
	// 0x1: Key event
	// 0x2: Mouse Event
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
	Data [10]uint16
}

type InputRecords [8]InputRecord

func (irs *InputRecords) Translate(buf []byte, signalChan chan os.Signal, eventsRead int) (int, error) {
	var bufferIndex int
	for i := range eventsRead {
		record := irs[i]
		num, err := record.Translate(buf[bufferIndex:], signalChan)
		if err != nil {
			return 0, err
		}
		bufferIndex += num
	}
	return bufferIndex, nil
}

func (ir *InputRecord) Translate(buf []byte, signalChan chan os.Signal) (int, error) {
	// TODO: fully create function to translate keypresses to buffer
	log.LogVf("reading: %v", ir.Data)
	switch ir.Type {
	case windows.KEY_EVENT: // key event
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
		return 1, nil
	case windows.WINDOW_BUFFER_SIZE_EVENT: // window buffer size event
		select {
		case signalChan <- ResizeSignal:
		default:
		}
	// TODO: handle mouse events
	case windows.MOUSE_EVENT: // mouse event
		if log.LogVerbose() {
			log.LogVf("Mouse event: %v", ir.Data)
		}
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
