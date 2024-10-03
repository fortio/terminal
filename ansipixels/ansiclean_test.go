package ansipixels

import (
	"testing"
)

var testCases = []struct {
	name     string
	input    string
	expected string
}{
	{
		"NoEscapeSequence",
		"Hello World, life is good, isn't it - is this long enough?",
		"Hello World, life is good, isn't it - is this long enough?",
	},
	{
		"UnterminatedEscapeSequence-1",
		"Hello World\x1b[1234",
		"Hello World",
	},
	{
		"UnterminatedEscapeSequence-2",
		"Hello World\x1b[",
		"Hello World",
	},
	{
		"ShortestEscapeSequenceAtEnd",
		"Hello Woooo\x1b[m",
		"Hello Woooo",
	},
	{
		"ShortestEscapeSequence",
		"\x1b[m",
		"",
	},
	{
		"SingleEscapeSequence",
		"Hello \x1b[31mWorld\x1b[0m cruel.",
		"Hello World cruel.",
	},
	{
		"MultipleEscapeSequences",
		"\x1b[31mHello\x1b[0m \x1b[32mWorld\x1b[0m tada!",
		"Hello World tada!",
	},
	/* we don't need mouse escape clean up anymore as we parse the mouse escape sequences */
	/*
		{
			"WithMouseEscapeSequence",
			"Hello \x1b[MCqGMouse",
			"Hello Mouse",
		},
		{
			"UnterminatedMouseEscapeSequence1",
			"Hello \x1b[MCq",
			"Hello ",
		},
		{
			"UnterminatedMouseEscapeSequence2",
			"Hello \x1b[MC",
			"Hello ",
		},
		{
			"UnterminatedMouseEscapeSequence3",
			"Hello \x1b[M",
			"Hello ",
		},
	*/
}

func TestAnsiCleanRE(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := ansiCleanRE(tc.input)
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestAnsiCleanHR(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := AnsiClean([]byte(tc.input))
			if string(actual) != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func BenchmarkAnsiCleanRE(b *testing.B) {
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ansiCleanRE(tc.input)
			}
		})
	}
}

func BenchmarkAnsiCleanHR(b *testing.B) {
	for _, tc := range testCases {
		inp := []byte(tc.input)
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				AnsiClean(inp)
			}
		})
	}
}
