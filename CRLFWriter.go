package terminal

import (
	"bytes"
	"io"
)

type CRLFWriter struct {
	// Out is the underlying writer to write to.
	Out io.Writer
}

func (w *CRLFWriter) Write(buf []byte) (n int, err error) {
	// Someone copied from x/term's writeWithCRLF
	for len(buf) > 0 {
		i := bytes.IndexByte(buf, '\n')
		todo := len(buf)
		if i >= 0 {
			todo = i
		}
		var nn int
		nn, err = w.Out.Write(buf[:todo])
		n += nn
		if err != nil {
			return n, err
		}
		buf = buf[todo:]
		if i >= 0 {
			if _, err = w.Out.Write([]byte{'\r', '\n'}); err != nil {
				return n, err
			}
			n++
			buf = buf[1:]
		}
	}
	return n, nil
}
