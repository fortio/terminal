package ansipixels

import "bytes"

func (ap *AnsiPixels) MouseClickOn() {
	ap.WriteString("\033[?1000h")
}

func (ap *AnsiPixels) MouseClickOff() {
	ap.WriteString("\033[?1000l")
}

func (ap *AnsiPixels) MouseTrackingOn() {
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

func (ap *AnsiPixels) MouseDecode() {
	ap.Mouse = false
	idx := bytes.Index(ap.Data, mouseDataPrefix)
	if idx == -1 {
		return
	}
	start := idx + len(mouseDataPrefix)
	if start+3 > len(ap.Data) {
		return
	}
	b := ap.Data[start]
	x := ap.Data[start+1]
	y := ap.Data[start+2]
	ap.Data = append(ap.Data[:idx], ap.Data[start+3:]...)
	ap.Mx = int(x) - 32
	ap.My = int(y) - 32
	ap.Mbuttons = int(b) - 32
	ap.Mouse = true
	ap.MouseDecode()
}
