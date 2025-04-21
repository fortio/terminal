package ansipixels

import (
	"testing"
)

var testCases = []struct {
	name     string
	input    string
	expected string
	delta    string
}{
	{
		"Empty",
		"",
		"",
		"",
	},
	{
		"EndsWithJustESC",
		"123\x1b",
		"123",
		"\x1b",
	},
	{
		"SeqPlusEndsWithJustESC",
		"1\x1b[36m23\x1b",
		"123",
		"\x1b",
	},
	{
		"AnotherUnterminatedSimple",
		"1\x1b[36m2\x1b[0",
		"12",
		"\x1b[0",
	},
	{
		"AnotherUnterminated",
		"lue\n\x1b[36m    \x1b[0",
		"lue\n    ",
		"\x1b[0",
	},
	{
		"OneGoodOneUnterminated",
		"\x1b[35mcommand \x1b[0",
		"command ",
		"\x1b[0",
	},
	{
		"NoEscapeSequence",
		"Hello World, life is good, isn't it - is this long enough?",
		"Hello World, life is good, isn't it - is this long enough?",
		"",
	},
	{
		"UnterminatedEscapeSequence-1",
		"Hello World\x1b[1234",
		"Hello World",
		"\x1b[1234",
	},
	{
		"UnterminatedEscapeSequence-2",
		"Hello World\x1b[",
		"Hello World",
		"\x1b[",
	},
	{
		"ShortestEscapeSequenceAtEnd",
		"Hello Woooo\x1b[m",
		"Hello Woooo",
		"",
	},
	{
		"ShortestEscapeSequence",
		"\x1b[m",
		"",
		"",
	},
	{
		"SingleEscapeSequence",
		"Hello \x1b[31mWorld\x1b[0m cruel.",
		"Hello World cruel.",
		"",
	},
	{
		"MultipleEscapeSequences",
		"\x1b[31mHello\x1b[0m \x1b[32mWorld\x1b[0m tada!",
		"Hello World tada!",
		"",
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

func TestAnsiCleanHR(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bInp := []byte(tc.input)
			actual, leftoverIdx := AnsiClean(bInp)
			if string(actual) != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
			if leftoverIdx > len(bInp) {
				t.Fatalf("leftoverIdx %d > len(bInp) %d", leftoverIdx, len(bInp))
			}
			expectedDelta := bInp[leftoverIdx:]
			if string(expectedDelta) != tc.delta {
				t.Errorf("expected delta %q, got %q", tc.delta, expectedDelta)
			}
		})
	}
}

func BenchmarkAnsiCleanRE(b *testing.B) {
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for range b.N {
				ansiCleanRE(tc.input)
			}
		})
	}
}

func BenchmarkAnsiCleanHR(b *testing.B) {
	for _, tc := range testCases {
		inp := []byte(tc.input)
		b.Run(tc.name, func(b *testing.B) {
			for range b.N {
				AnsiClean(inp)
			}
		})
	}
}
