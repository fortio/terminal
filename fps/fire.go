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
	for i := 0; i < 256; i++ {
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
	for x := 0; x < f.w; x++ {
		f.Set(x, f.h-1, 255)
		// Show/debug the palette:
		// f.Set(x, f.h-2, safecast.MustConvert[byte]((255*x+1)/f.w))
	}
	f.on = true
}

// Turn off the fire at the bottom
func (f *FireState) Off() {
	for x := 0; x < f.w; x++ {
		f.Set(x, f.h-1, 1)
	}
	f.on = false
}

func (f *FireState) Update() {
	for y := f.h - 2; y >= 0; y-- {
		for x := 0; x < f.w; x++ {
			r := rand.IntN(3) - 1
			v := f.At((x+r+f.w)%f.w, y+1)
			pv := f.At(x, y)
			newV := byte(max(0, (int(pv)+2*(int(v)-int(rand.Float32()*4.*255./(float32(f.h-1)))))/3))
			prev := f.Set(x, y, newV)
			if prev != 0 && newV == 0 {
				f.Set(x, y, 1)
			}
		}
	}
}

func (f *FireState) Render(ap *ansipixels.AnsiPixels) {
	for y := 0; y < f.h; y++ {
		first := true
		prevX := -1
		for x := 0; x < f.w; x++ {
			v := f.At(x, y)
			if v == 0 {
				continue
			}
			prevX = x
			if first {
				ap.MoveCursor(x+ap.Margin, y+ap.Margin)
				first = false
			} else {
				if x-prevX > 1 {
					ap.MoveHorizontally(x)
				}
			}
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
