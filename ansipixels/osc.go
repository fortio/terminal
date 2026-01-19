package ansipixels

// See https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking

import (
	"bytes"

	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/terminal/ansipixels/tcolor"
)

// SyncBackgroundColor will synchronously request and read the terminal background color.
// It also sets Transparency to true such as image display that have alpha channel
// or the background color set to same color will use transparency (by not outputting
// any color or pixels for 0 alpha and pixels matching the background color).
func (ap *AnsiPixels) SyncBackgroundColor() bool {
	if ap.GotBackground {
		return true
	}
	ap.RequestBackgroundColor()
	ap.Out.Flush()
	_ = ap.ReadOrResizeOrSignal()
	ap.Transparency = ap.OSCDecode()
	return ap.GotBackground // Same as ap.Transparency now.
}

// RequestBackgroundColor sends a request to the terminal to return the current
// background color. Which we can use to blend images and pixels with said background.
// Use [SyncBackgroundColor] to do both the request and read/decode in one call, this
// low level function just does the request part and only for fortio.org/tev decoding.
func (ap *AnsiPixels) RequestBackgroundColor() {
	ap.WriteString("\033]11;?\x07")
	ap.backgroundRequested = true
}

const osc11ReplyPrefix = "\033]11;rgb:"

// OSCDecode decodes a single OSC reply from the AnsiPixels.Data buffer.
// It is automatically called through [MouseDecodeAll] by [ReadOrResizeOrSignal] and [ReadOrResizeOrSignalOnce]
// unless NoDecode is set to true
// (so you typically don't need to call it directly and can just check the BackgroundColor property).
// It doesn't do anything unless [RequestBackgroundColor] was called first. Use [SyncBackgroundColor] to do both.
func (ap *AnsiPixels) OSCDecode() bool {
	if !ap.backgroundRequested {
		return ap.GotBackground
	}
	ap.Mouse = false
	idx := bytes.Index(ap.Data, []byte(osc11ReplyPrefix))
	if idx == -1 {
		// log.Debugf("OSCDecode: no OSC 11 reply prefix (%q) found in %q", string(osc11ReplyPrefix), ap.Data)
		return false
	}
	start := idx + len(osc11ReplyPrefix)
	// Scan and parse
	log.LogVf("OSCDecode: found prefix at %d, start at %d, data len %d: %q", idx, start, len(ap.Data), ap.Data[start:])
	i := start
	done := false
	var r, g, b, endIdx int
	state := 0
	// Fast no alloc parsing (vs `11\h+;\h+;\h+(\007|\033\\)` regexp)
	dataLen := len(ap.Data)
	for !done {
		if i >= dataLen {
			log.LogVf("OSCDecode: partial OSC event %q", ap.Data[start:])
			return false
		}
		c := ap.Data[i]
		switch {
		case c == '\033':
			// Check if next character is actually a backslash
			if i+1 < dataLen && ap.Data[i+1] == '\\' {
				i++ // we got the expected backslash
			} else {
				log.Errf("OSCDecode: expected '\\' after ESC at %d in %q", i, ap.Data[start:])
				return false
			}
			fallthrough
		case c == '\007':
			endIdx = i + 1
			done = true
		case c == '/':
			state++
			if state > 2 {
				log.Errf("OSCDecode: too many / found %d %q", i, ap.Data[start:])
				return false
			}
		case (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f'):
			// Parse the r, g, and b colors
			// clear the lowercase bit
			if c >= 'a' {
				c -= 0x20
			}
			v := int(c - '0')
			if c >= 'A' {
				v = int(c-'A') + 10
			}
			// log.Debugf("OSCDecode: found color digit %d (%c) at %d", v, c, i)
			switch state {
			case 0:
				r = 16*r + v
			case 1:
				g = 16*g + v
			case 2:
				b = 16*b + v
			}
		default:
			log.Errf("OSC decode: unexpected OSC data byte at %d %q", i, ap.Data[start:])
			return false
		}
		i++
	}
	if state != 2 {
		log.Errf("OSC decode: not enough / found %d %q", i, ap.Data[start:])
		return false
	}
	ap.Background = tcolor.RGBColor{
		R: safecast.MustConv[uint8](r >> 8),
		G: safecast.MustConv[uint8](g >> 8),
		B: safecast.MustConv[uint8](b >> 8),
	}
	log.LogVf("OSC decode: found data %q - <r:%x g:%x b:%x> -> %s", ap.Data[start:endIdx], r, g, b, ap.Background)
	ap.Data = append(ap.Data[:idx], ap.Data[endIdx:]...)
	ap.GotBackground = true
	ap.backgroundRequested = false
	return true
}
