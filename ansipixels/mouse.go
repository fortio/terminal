package ansipixels

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
