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

var v2col [256]string

func init() {
	for i := range 256 {
		/*
			r := min(255, 4*i)                // Red increases quickly and dominates
			g := min(255, max(0, (i-96)*5))   // Green starts later and rises slower for more orange
			b := min(255, max(0, (i-224)*12)) // Blue comes in very late, ensuring more orange and less yellow
		*/
		r := min(255, 4*i)
		g := min(255, max(0, (i-64)*3))
		b := min(255, max(0, (i-192)*6))
		/*
			r := min(255, 3*i)               // Red ramps up quickly
			g := min(255, max(0, (i-64)*3))  // Green starts increasing earlier to transition to orange/yellow
			b := min(255, max(0, (i-192)*4)) // Blue only increases near i = 192
		*/
		v2col[i] = fmt.Sprintf("\033[38;2;%d;%d;%dm█", r, g, b)
	}
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
		// f.Set(x, f.h-2, safecast.MustConvert[byte]((255*x+1)/f.w))
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
				ap.MoveHorizontally(x)
			}
			prevX = x
			ap.WriteString(v2col[v])
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
