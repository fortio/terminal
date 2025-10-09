package ansipixels

// See https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking

import (
	"bytes"
	"strings"

	"fortio.org/log"
)

// MouseClickOn turns on mouse click and wheel tracking.
// It will set decoded Mx, My, MButtons, Mouse flag etc... and call OnMouse.
// If you call *On do call *Off in your defer restore.
func (ap *AnsiPixels) MouseClickOn() {
	// https://github.com/ghostty-org/website/blob/main/docs/vt/csi/xtshiftescape.mdx
	// Let us see shift key modifiers:
	ap.WriteString("\033[>1s")
	// Set the SGR mouse mode (SGR = Select Graphic Rendition) to be able to get coordinates
	// past 95 in windows terminal and past 223 for everyone else:
	ap.WriteString("\033[?1006h")
	ap.WriteString("\033[?1000h")
}

func (ap *AnsiPixels) MouseClickOff() {
	ap.WriteString("\033[?1000l")
}

// MouseTrackingOn turns on tracking for mouse movements and click tracking.
// It will set decoded Mx, My, MButtons, Mouse flag etc... and call OnMouse.
// If you call *On do call *Off in your defer restore.
func (ap *AnsiPixels) MouseTrackingOn() {
	// https://ghostty.org/docs/vt/csi/xtshiftescape
	// Let us see shift key modifiers:
	ap.WriteString("\033[>1s")
	// Set the SGR mouse mode (SGR = Select Graphic Rendition) to be able to get coordinates
	// past 95 in windows terminal and past 223 for everyone else:
	ap.WriteString("\033[?1006h")
	ap.WriteString("\033[?1003h")
}

func (ap *AnsiPixels) MouseTrackingOff() {
	ap.WriteString("\033[?1003l")
}

func (ap *AnsiPixels) MouseX10Off() {
	ap.WriteString("\033[?9l")
}

// MouseX10On is the X10 mouse mode and is here for reference but is not automatically decoded for you.
func (ap *AnsiPixels) MouseX10On() {
	ap.WriteString("\033[?9h")
}

// MousePixelsOn will report coordinates (Mx, My) in pixels instead of cells.
func (ap *AnsiPixels) MousePixelsOn() {
	ap.WriteString("\x1b[?1016h")
}

func (ap *AnsiPixels) MousePixelsOff() {
	ap.WriteString("\x1b[?1016l")
}

var sgrMouseDataPrefix = []byte{0x1b, '[', '<'}

type MouseStatus int

const (
	NoMouse MouseStatus = iota
	MouseComplete
	MousePrefix
	MouseError
)

// MouseDecode decodes a single mouse data event from the AnsiPixels.Data buffer.
// It is automatically called through [MouseDecodeAll] by [ReadOrResizeOrSignal] and [ReadOrResizeOrSignalOnce]
// unless NoDecode is set to true
// (so you typically don't need to call it directly and can just check the Mouse, Mx, My, Mbuttons fields).
// If there is more than one event you can consume them by calling [MouseDecodeAll].
//
// It returns one of the MouseStatus values:
// - NoMouse if no mouse data was found
// - MouseComplete if the mouse data was successfully decoded
// - MousePrefix if the mouse data prefix was found but not enough data to decode it.
// That last one and the complexity don't seem to be occurring anymore but are used/expected by
// https://github.com/fortio/tev as it used to be required on windows.
func (ap *AnsiPixels) MouseDecode() MouseStatus {
	ap.Mouse = false
	idx := bytes.Index(ap.Data, sgrMouseDataPrefix)
	if idx == -1 {
		return NoMouse
	}
	start := idx + len(sgrMouseDataPrefix)
	// Scan and parse
	log.LogVf("MouseDecode: found prefix at %d, start at %d, data len %d: %q", idx, start, len(ap.Data), ap.Data[start:])
	i := start
	done := false
	buttonRelease := false
	var b, x, y, endIdx int
	state := 0
	// Fast no alloc parsing (vs `\d+;\d+;\d+[mM]` regexp)
	dataLen := len(ap.Data)
	for !done {
		if i >= dataLen {
			log.LogVf("MouseDecode: partial mouse event %q", ap.Data[start:])
			return MousePrefix
		}
		c := ap.Data[i]
		switch c {
		case 'm':
			buttonRelease = true
			fallthrough
		case 'M':
			endIdx = i + 1
			done = true
		case ';':
			state++
			if state > 2 {
				log.Errf("MouseDecode: too many ; found %d %q", i, ap.Data[start:])
				return MouseError
			}
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': // or case c>='0' && c<='9':

			// Parse the button, x, and y coordinates
			switch state {
			case 0:
				b = 10*b + int(c-'0')
			case 1:
				x = 10*x + int(c-'0')
			case 2:
				y = 10*y + int(c-'0')
			}
		default:
			log.Errf("MouseDecode: unexpected mouse data byte at %d %q", i, ap.Data[start:])
			return MouseError
		}
		i++
	}
	if state != 2 {
		log.Errf("MouseDecode: not enough ; found %d %q", i, ap.Data[start:])
		return MouseError
	}
	log.LogVf("MouseDecode: found mouse data %q - release: %t", ap.Data[start:endIdx], buttonRelease)
	ap.Data = append(ap.Data[:idx], ap.Data[endIdx:]...)
	ap.Mx = x
	ap.My = y
	ap.Mbuttons = b
	ap.Mrelease = buttonRelease
	ap.Mouse = true
	return MouseComplete
}

// MouseDecodeAll decodes all mouse events available,
// useful at low fps where we may have multiple mouse events.
// Internally used when ap.NoDecode is false.
// The last event if more than one has been accumulated/sent will win.
// If you need to keep track of each you can have an OnMouse callback.
// Or set NoDecode to true and call MouseDecode yourself.
func (ap *AnsiPixels) MouseDecodeAll() {
	gotMouse := false
	for ap.MouseDecode() == MouseComplete {
		// keep decoding mouse events until we have no more.
		// TODO: consider keeping a list of all the events or saving the left/right clicks in priority
		// over mouse movements.
		gotMouse = true
		if ap.OnMouse != nil {
			ap.OnMouse()
		}
	}
	ap.OSCDecode()
	ap.Mouse = gotMouse
}

const (
	MouseLeft       = 0b00
	MouseMiddle     = 0b01
	MouseRight      = 0b10
	MouseMove       = 0b100000
	MouseWheelUp    = 0b1000000
	MouseWheelDown  = 0b1000001
	Shift           = 0b000100
	Alt             = 0b001000
	Ctrl            = 0b010000
	AllModifiers    = Shift | Alt | Ctrl
	AnyModifierMask = ^AllModifiers
	// MouseWheelMask is what is used to identify a mouse wheel event.
	// On a mac with a physical mouse, shift mousewheel is translated to button 6,7 which
	// here looks like we set the MouseRight bit (when shift-mousewheeling).
	MouseWheelMask = ^(AllModifiers | MouseRight)
)

// MouseDebugString returns a string representation of the current mouse buttons and modifier state.
// See https://github.com/fortio/tev for an event debugging tool that uses this.
func (ap *AnsiPixels) MouseDebugString() string {
	if !ap.Mouse {
		return "No mouse event "
	}
	buf := strings.Builder{}
	if ap.AltMod() {
		buf.WriteString("Alt ")
	}
	if ap.ShiftMod() {
		buf.WriteString("Shift ")
	}
	if ap.CtrlMod() {
		buf.WriteString("Ctrl ")
	}
	if ap.LeftClick() {
		buf.WriteString("LeftClick ")
	}
	if ap.RightClick() {
		buf.WriteString("RightClick ")
	}
	if ap.Middle() {
		buf.WriteString("MiddleClick ")
	}
	if ap.LeftDrag() {
		buf.WriteString("LeftDrag ")
	}
	if ap.RightDrag() {
		buf.WriteString("RightDrag ")
	}
	if ap.MiddleDrag() {
		buf.WriteString("MiddleDrag ")
	}
	if ap.MouseWheelUp() {
		buf.WriteString("MouseWheelUp ")
	}
	if ap.MouseWheelDown() {
		buf.WriteString("MouseWheelDown ")
	}
	if ap.MouseRelease() {
		buf.WriteString("Released ")
	}
	return buf.String()
}

func (ap *AnsiPixels) MouseWheelUp() bool {
	return ap.Mouse && ((ap.Mbuttons & MouseWheelMask) == MouseWheelUp)
}

func (ap *AnsiPixels) MouseWheelDown() bool {
	return ap.Mouse && ((ap.Mbuttons & MouseWheelMask) == MouseWheelDown)
}

func (ap *AnsiPixels) AltMod() bool {
	return ap.Mbuttons&Alt != 0
}

func (ap *AnsiPixels) ShiftMod() bool {
	return ap.Mbuttons&Shift != 0
}

func (ap *AnsiPixels) CtrlMod() bool {
	return ap.Mbuttons&Ctrl != 0
}

func (ap *AnsiPixels) AnyModifier() bool {
	return ap.Mbuttons&AllModifiers != 0
}

func (ap *AnsiPixels) LeftClick() bool {
	return ap.Mouse && ((ap.Mbuttons & AnyModifierMask) == MouseLeft)
}

func (ap *AnsiPixels) Middle() bool {
	return ap.Mouse && ((ap.Mbuttons & AnyModifierMask) == MouseMiddle)
}

func (ap *AnsiPixels) RightClick() bool {
	return ap.Mouse && ((ap.Mbuttons & AnyModifierMask) == MouseRight)
}

func (ap *AnsiPixels) LeftDrag() bool {
	return ap.Mouse && ((ap.Mbuttons & AnyModifierMask) == MouseMove|MouseLeft)
}

func (ap *AnsiPixels) MiddleDrag() bool {
	return ap.Mouse && ((ap.Mbuttons & AnyModifierMask) == MouseMove|MouseMiddle)
}

func (ap *AnsiPixels) RightDrag() bool {
	return ap.Mouse && ((ap.Mbuttons & AnyModifierMask) == MouseMove|MouseRight)
}

func (ap *AnsiPixels) MouseRelease() bool {
	return ap.Mouse && ap.Mrelease
}
