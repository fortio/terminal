package terminal_test

import (
	"strings"
	"testing"

	"fortio.org/terminal"
)

type testCRWriter struct {
	count   int
	builder strings.Builder
}

func (t *testCRWriter) Write(p []byte) (n int, err error) {
	t.count++
	return t.builder.Write(p)
}

func TestCRWriter_Write(t *testing.T) {
	tests := []struct {
		input     string
		want      string
		numWrites int
	}{
		{"Hello, World!", "Hello, World!", 1}, // no \n only 1 write
		{"", "", 0},                           // empty string, no write
		{"\n", "\r\n", 1},                     // 1 \n optimized to 1 write "at the end"
		{"Hello, World!\n", "Hello, World!\r\n", 1},
		{"Hello,\nWorld!\nNo last newline", "Hello,\r\nWorld!\r\nNo last newline", 1},
		{"\nHello, World!\n", "\r\nHello, World!\r\n", 1},
		{"Hello, World!\n\t", "Hello, World!\r\n\t", 1},
	}
	for _, tt := range tests {
		out := testCRWriter{}
		w := &terminal.CRWriter{Out: &out}
		n, err := w.Write([]byte(tt.input))
		if err != nil {
			t.Errorf("CRWriter.Write(%q) error = %v, want nil", tt.input, err)
		}
		if n != len(tt.input) {
			t.Errorf("CRWriter.Write(%q) = %v, want %v", tt.input, n, len(tt.input))
		}
		if out.count != tt.numWrites {
			t.Errorf("CRWriter.Write(%q) = %v writes, want %v", tt.input, out.count, tt.numWrites)
		}
		actual := out.builder.String()
		if actual != tt.want {
			t.Errorf("CRWriter.Write(%q) = %q, want %q", tt.input, actual, tt.want)
		}
	}
}
