[![Go Reference](https://pkg.go.dev/badge/fortio.org/terminal.svg)](https://pkg.go.dev/fortio.org/terminal)
# terminal

Fortio's terminal is a `readline` style library. It handles prompts, edit (like Ctrl-A for beginning of line etc...), navigating through history using arrow keys, loading and saving history from file, etc... It works on everywhere go does (including macOS, Windows (using Terminal app), Linux).

See [example/main.go](example/main.go) for a rather complete example/demo.

See the godoc above for details.

The [grol](https://github.com/grol-io/grol#grol) command line repl and others use this.

The implementations currently is a wrapper fully encapsulating (our fork of) [x/term](https://github.com/golang/term), i.e. [fortio.org/term](https://github.com/fortio/term) and new features like the interrupts handling (filters Ctrl-C ahead of term' reads)

## FPS

There is also a new [ansipixels](https://pkg.go.dev/fortio.org/terminal/ansipixels) package for drawing on the terminal and the tagged release also include `fps` that uses that package to test your terminal frames per second capabilities.
See the source [ansipixels/fps/fps.go](ansipixels/fps/fps.go)

You can get the binary from [releases](https://github.com/fortio/terminal/releases)

Or just run
```
CGO_ENABLED=0 go install fortio.org/terminal/ansipixels/fps@latest  # to install or just
CGO_ENABLED=0 go run fortio.org/terminal/ansipixels/fps@latest  # to run without install
```

or even
```
docker run -ti fortio/fps # but that's obviously slower
```

or
```
brew install fortio/tap/fps
```

Use the `-image` flag to pass a different image to load as background. Or use `-i` and fps is now just a terminal image viewer.

Pass an optional `maxfps` as argument.

E.g `fps -image my.jpg 60` will run at 60 fps with `my.jpg` as background.

After hitting any key to start the measurement, you can also resize the window at any time and fps will render with the new size.
Use `q` to stop.

![fps screenshot](fps_sshot.png)

Image viewer screenshot:

![fps image viewer](fps_image_viewer.png)

Detailed statistics are saved in a JSON files and can be visualized or compared by running [fortio report](https://github.com/fortio/fortio#installation)

![fps fortio histogram](histogram.png)

### Usage

Additional flags/command help:
```
fps v0.18.0 usage:
	fps [flags] [maxfps] or fps -i imagefiles...
or 1 of the special arguments
	fps {help|envhelp|version|buildinfo}
flags:
  -color
    	If your terminal supports color, this will load image in (216) colors instead of monochrome (default true)
  -gray
    	Convert the image to grayscale
  -i	Arguments are now images files to show, no FPS test (hit any key to continue)
  -image string
    	Image file to display in monochrome in the background instead of the default one
  -n number of frames
    	Start immediately an FPS test with the specified number of frames (default is interactive)
  -nobox
    	Don't draw the box around the image, make the image full screen instead of 1 pixel less on all sides
  -truecolor
    	If your terminal supports truecolor, this will load image in truecolor (24bits) instead of monochrome
```
