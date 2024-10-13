package main

import (
	"fmt"
	"math/rand/v2"

	"fortio.org/terminal/ansipixels"
)

type FireState struct {
	h      int
	w      int
	buffer []byte
	on     bool
}

var fire *FireState

var v2colTrueColor [256]string
var v2col256 [256]string

func init() {
	for i := range 256 {
		r := min(255, 4*i)
		g := min(255, max(0, (i-64)*3))
		b := min(255, max(0, (i-192)*6))
		v2colTrueColor[i] = fmt.Sprintf("\033[38;2;%d;%d;%dm█", r, g, b)
	}
	v2col256[0] = "\033[38;5;16m█"   // ansi 216 black.
	v2col256[1] = "\033[38;5;52m█"   // #5f0000
	v2col256[2] = "\033[38;5;88m█"   // #870000
	v2col256[3] = "\033[38;5;124m█"  // #af0000
	v2col256[4] = "\033[38;5;160m█"  // #d70000
	v2col256[5] = "\033[38;5;196m█"  // #ff0000
	v2col256[6] = "\033[38;5;166m█"  // #d75f00
	v2col256[7] = "\033[38;5;202m█"  // #ff5f00
	v2col256[8] = "\033[38;5;208m█"  // #ff8700
	v2col256[9] = "\033[38;5;214m█"  // #ffaf00
	v2col256[10] = "\033[38;5;215m█" // #ffaf5f
	v2col256[11] = "\033[38;5;220m█" // #ffd700
	v2col256[12] = "\033[38;5;221m█" // #ffd75f
	v2col256[13] = "\033[38;5;226m█" // #ffff00
	v2col256[14] = "\033[38;5;227m█" // #ffff5f
	v2col256[15] = "\033[38;5;231m█" // #ffffff
}

func InitFire(ap *ansipixels.AnsiPixels) *FireState {
	f := &FireState{h: ap.H - 2*ap.Margin, w: ap.W - 2*ap.Margin}
	f.buffer = make([]byte, f.h*f.w)
	f.Start()
	return f
}

func ToggleFire() {
	if fire == nil {
		return
	}
	if fire.on {
		fire.Off()
	} else {
		fire.Start()
	}
}

func (f *FireState) At(x, y int) byte {
	return f.buffer[y*f.w+x]
}

func (f *FireState) Set(x, y int, v byte) byte {
	idx := y*f.w + x
	prev := f.buffer[idx]
	f.buffer[idx] = v
	return prev
}

func (f *FireState) Start() {
	for x := range f.w {
		f.Set(x, f.h-1, 255)
		// Show/debug the palette:
		// f.Set(x, f.h-3, safecast.MustConvert[byte](256-(255*(x+1))/f.w))
		// f.Set(x, f.h-2, safecast.MustConvert[byte]((255*(x+1))/f.w))
	}
	f.on = true
}

// Turn off the fire at the bottom.
func (f *FireState) Off() {
	for x := range f.w {
		f.Set(x, f.h-1, 1)
	}
	f.on = false
}

func (f *FireState) Update() {
	for y := f.h - 2; y >= 0; y-- {
		for x := range f.w {
			r := rand.IntN(3) - 1 //nolint:gosec // this _is_ randv2!
			v := f.At((x+r+f.w)%f.w, y+1)
			pv := f.At(x, y)
			newV := byte(max(0, (float32(pv)+4*(float32(v)-rand.Float32()*4.*255./(float32(f.h-1))))/5.)) //nolint:gosec // this _is_ randv2!
			prev := f.Set(x, y, newV)
			if prev != 0 && newV == 0 {
				f.Set(x, y, 1)
			}
		}
	}
}

func (f *FireState) Render(ap *ansipixels.AnsiPixels) {
	for y := range f.h {
		first := true
		prevX := -999
		for x := range f.w {
			v := f.At(x, y)
			if v == 0 {
				continue
			}
			switch {
			case first:
				ap.MoveCursor(x+ap.Margin, y+ap.Margin)
				first = false
			case x != prevX+1:
				ap.MoveHorizontally(x + ap.Margin)
			}
			prevX = x
			if ap.TrueColor {
				ap.WriteString(v2colTrueColor[v])
			} else {
				ap.WriteString(v2col256[v/16])
			}
		}
	}
}

func AnimateFire(ap *ansipixels.AnsiPixels, frame int64) {
	if frame == 0 {
		fire = InitFire(ap)
	}
	fire.Update()
	fire.Render(ap)
}
