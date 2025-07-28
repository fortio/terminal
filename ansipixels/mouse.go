package ansipixels

import (
	"bytes"

	"fortio.org/log"
)

func (ap *AnsiPixels) MouseClickOn() {
	// https://github.com/ghostty-org/ghostty/blame/main/website/app/vt/xtshiftescape/page.mdx
	// Let us see shift key modifiers:
	ap.WriteString("\033[>1s")
	ap.WriteString("\033[?1000h")
}

func (ap *AnsiPixels) MouseClickOff() {
	ap.WriteString("\033[?1000l")
}

func (ap *AnsiPixels) MouseTrackingOn() {
	// https://github.com/ghostty-org/ghostty/blame/main/website/app/vt/xtshiftescape/page.mdx
	// Let us see shift key modifiers:
	ap.WriteString("\033[>1s")
	ap.WriteString("\033[?1003h")
}

func (ap *AnsiPixels) MouseTrackingOff() {
	ap.WriteString("\033[?1003l")
}

func (ap *AnsiPixels) MouseX10Off() {
	ap.WriteString("\033[?9l")
}

func (ap *AnsiPixels) MouseX10On() {
	ap.WriteString("\033[?9h")
}

func (ap *AnsiPixels) MousePixelsOn() {
	ap.WriteString("\x1b[?1016h")
}

func (ap *AnsiPixels) MousePixelsOff() {
	ap.WriteString("\x1b[?1016l")
}

var mouseDataPrefix = []byte{0x1b, '[', 'M'}

// MouseDecode decodes the mouse data from the AnsiPixels.Data buffer.
// It us automatically called by ReadOrResizeOrSignal and ReadOrResizeOrSignalOnce unless
// NoDecode is set to true (so you typically don't need to call it directly and can just
// check the Mouse, Mx, My, Mbuttons fields).
// If there is more than one event you can consume them by calling
//
//	for ap.MouseDecode() {}
//
// (until it returns false, it returns true if there was something decoded).
func (ap *AnsiPixels) MouseDecode() bool {
	ap.Mouse = false
	idx := bytes.Index(ap.Data, mouseDataPrefix)
	if idx == -1 {
		return false
	}
	start := idx + len(mouseDataPrefix)
	if start+3 > len(ap.Data) {
		// Read the missing bytes (eg windows terminal sends in 2 chunks).
		need := start + 3 - len(ap.Data)
		buf := [3]byte{}
		n, err := ap.SharedInput.TR.Read(buf[:need])
		if err != nil {
			log.Errf("Error reading additional mouse data: %v", err)
			return false
		}
		ap.Data = append(ap.Data, buf[:n]...)
		if n < need {
			log.Errf("Not enough bytes read for mouse data: %d, expected %d", n, need)
			return false
		}
	}
	b := ap.Data[start]
	x := ap.Data[start+1]
	y := ap.Data[start+2]
	ap.Data = append(ap.Data[:idx], ap.Data[start+3:]...)
	ap.Mx = int(x) - 32
	ap.My = int(y) - 32
	ap.Mbuttons = int(b) - 32
	ap.Mouse = true
	return true
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
	// On a mac with a physical mouse, shift mousewheel is translated to button 6,7 which
	// here looks like we set the MouseRight bit (when shift-mousewheeling).
	MouseWheelMask = ^(AllModifiers | MouseRight)
)

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
	return ap.Mbuttons&Alt != 0
}

func (ap *AnsiPixels) CtrlMod() bool {
	return ap.Mbuttons&Alt != 0
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
