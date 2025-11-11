package table

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"fortio.org/terminal/ansipixels"
)

func TestCreateTableLines_LeftAlignment(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Left, Left, Left}
	columnSpacing := 2
	table := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "NYC"},
		{"Bob", "25", "LA"},
	}

	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// Check that all lines have the same width (padded properly)
	for i, line := range lines {
		lineWidth := ap.ScreenWidth(line)
		if lineWidth != width {
			t.Errorf("Line %d has width %d, expected %d: %q", i, lineWidth, width, line)
		}
	}

	// Check left alignment - content should be at the start of each column
	if !strings.HasPrefix(lines[0], "Name") {
		t.Errorf("First column should start with 'Name', got: %q", lines[0])
	}
}

func TestCreateTableLines_RightAlignment(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Right, Right, Right}
	columnSpacing := 2
	table := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "NYC"},
		{"Bob", "25", "LA"},
	}

	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// Check that all lines have the same width
	for i, line := range lines {
		lineWidth := ap.ScreenWidth(line)
		if lineWidth != width {
			t.Errorf("Line %d has width %d, expected %d: %q", i, lineWidth, width, line)
		}
	}

	// With right alignment, shorter values should be padded on the left
	// "Bob" should have more leading spaces than "Alice" in the first column
	bobLine := lines[2]
	aliceLine := lines[1]

	// Find where content starts (after leading spaces)
	bobStart := strings.Index(bobLine, "Bob")
	aliceStart := strings.Index(aliceLine, "Alice")

	if bobStart <= aliceStart {
		t.Errorf("Bob should have more leading spaces than Alice with right alignment")
	}
}

func TestCreateTableLines_CenterAlignment(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Center, Center, Center}
	columnSpacing := 2
	table := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "NYC"},
		{"Bob", "25", "LA"},
	}

	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// Check that all lines have the same width
	for i, line := range lines {
		lineWidth := ap.ScreenWidth(line)
		if lineWidth != width {
			t.Errorf("Line %d has width %d, expected %d: %q", i, lineWidth, width, line)
		}
	}

	// With center alignment, content should be roughly centered
	// "Bob" (3 chars) in a column sized for "Alice" (5 chars) should have 1 space before
	bobLine := lines[2]
	bobStart := strings.Index(bobLine, "Bob")

	// Should have at least some leading space (centered)
	if bobStart == 0 {
		t.Errorf("Bob should be centered with leading space, got line: %q", bobLine)
	}
}

func TestCreateTableLines_MixedAlignment(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Left, Right, Center}
	columnSpacing := 3
	table := [][]string{
		{"Product", "Price", "Stock"},
		{"Apple", "1.50", "100"},
		{"Banana", "0.75", "50"},
	}

	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// Verify all lines have consistent width
	for i, line := range lines {
		lineWidth := ap.ScreenWidth(line)
		if lineWidth != width {
			t.Errorf("Line %d has width %d, expected %d: %q", i, lineWidth, width, line)
		}
	}

	// Check that lines are not empty
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			t.Errorf("Line %d is empty or whitespace only", i)
		}
	}
}

func TestCreateTableLines_DifferentColumnSpacing(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Left, Left}
	table := [][]string{
		{"A", "B"},
		{"C", "D"},
	}

	testCases := []int{0, 1, 2, 5, 10}

	for _, spacing := range testCases {
		lines, width := CreateTableLines(ap, alignment, spacing, table, BorderNone)

		// Check that spacing is correctly applied
		// Width should be: max_col0_width + spacing + max_col1_width
		expectedWidth := 1 + spacing + 1 // Both columns have width 1
		if width != expectedWidth {
			t.Errorf("With spacing %d, expected width %d, got %d", spacing, expectedWidth, width)
		}

		// Verify lines have the correct width
		for i, line := range lines {
			lineWidth := ap.ScreenWidth(line)
			if lineWidth != width {
				t.Errorf("Spacing %d, line %d has width %d, expected %d: %q", spacing, i, lineWidth, width, line)
			}
		}
	}
}

func TestCreateTableLines_SingleColumn(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Center}
	columnSpacing := 2
	table := [][]string{
		{"Header"},
		{"Row1"},
		{"Row2"},
	}

	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// Width should be the max width of all cells
	expectedWidth := 6 // "Header" is 6 chars
	if width != expectedWidth {
		t.Errorf("Expected width %d, got %d", expectedWidth, width)
	}
}

func TestCreateTableLines_UnevenColumnWidths(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Left, Left, Left}
	columnSpacing := 2
	table := [][]string{
		{"Short", "Medium Length", "X"},
		{"A", "B", "Very Long Column"},
		{"Test", "Data", "C"},
	}

	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// All lines should have the same width
	for i, line := range lines {
		lineWidth := ap.ScreenWidth(line)
		if lineWidth != width {
			t.Errorf("Line %d has width %d, expected %d: %q", i, lineWidth, width, line)
		}
	}

	// The width should accommodate the longest value in each column
	// Col 0: "Short" (5), Col 1: "Medium Length" (13), Col 2: "Very Long Column" (16)
	expectedWidth := 5 + 2 + 13 + 2 + 16
	if width != expectedWidth {
		t.Errorf("Expected width %d, got %d", expectedWidth, width)
	}
}

func TestCreateTableLines_EmptyTable(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Left}
	columnSpacing := 2
	table := [][]string{}

	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)

	if len(lines) != 0 {
		t.Errorf("Expected 0 lines for empty table, got %d", len(lines))
	}

	if width != 0 {
		t.Errorf("Expected width 0 for empty table, got %d", width)
	}
}

func TestCreateTableLines_InconsistentColumns(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Left, Left}
	columnSpacing := 2
	table := [][]string{
		{"A", "B"},
		{"C", "D", "E"}, // Extra column - should panic
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for inconsistent number of columns")
		}
	}()

	CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)
}

func TestCreateTableLines_CenterAlignmentOddEven(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Center}
	columnSpacing := 0

	// Test with even delta: 5 char column width - 3 char content = 2 char delta (even)
	tableEvenDelta := [][]string{
		{"ABCDE"},
		{"ABC"},
	}

	linesEvenDelta, _ := CreateTableLines(ap, alignment, columnSpacing, tableEvenDelta, BorderNone)

	// "ABC" centered in 5 chars with delta=2 should be " ABC " (1 space left, 1 space right)
	abcLine := linesEvenDelta[1]
	if !strings.HasPrefix(abcLine, " ABC ") {
		t.Errorf("Center alignment with even delta failed: expected ' ABC ', got %q", abcLine)
	}

	// Test with odd delta: 6 char column width - 3 char content = 3 char delta (odd)
	tableOddDelta := [][]string{
		{"ABCDEF"},
		{"ABC"},
	}

	linesOddDelta, _ := CreateTableLines(ap, alignment, columnSpacing, tableOddDelta, BorderNone)

	// "ABC" centered in 6 chars with delta=3 should be " ABC  " (1 space left, 2 spaces right due to odd delta)
	abcLineOdd := linesOddDelta[1]
	if !strings.HasPrefix(abcLineOdd, " ABC  ") {
		t.Errorf("Center alignment with odd delta failed: expected ' ABC  ', got %q", abcLineOdd)
	}
}

func TestCreateTableLines_ZeroColumnSpacing(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Left, Left, Left}
	columnSpacing := 0
	table := [][]string{
		{"A", "B", "C"},
		{"X", "Y", "Z"},
	}

	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)

	// Width should be sum of column widths with no spacing
	expectedWidth := 3 // 1 + 1 + 1
	if width != expectedWidth {
		t.Errorf("Expected width %d, got %d", expectedWidth, width)
	}

	// Lines should have adjacent columns with no spaces
	if lines[0] != "ABC" {
		t.Errorf("Expected 'ABC', got %q", lines[0])
	}
	if lines[1] != "XYZ" {
		t.Errorf("Expected 'XYZ', got %q", lines[1])
	}
}

func TestCreateTableLines_WithEmojisAndUnicode(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}
	alignment := []Alignment{Left, Center, Right}
	columnSpacing := 2
	table := [][]string{
		{"Name", "Icon", "Score"},
		{"Alice", "🎉", "100"},
		{"Bob", "🚀", "95"},
		{"Charlie", "✨", "98"},
	}

	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, BorderNone)

	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}

	// All lines should have consistent width
	for i, line := range lines {
		lineWidth := ap.ScreenWidth(line)
		if lineWidth != width {
			t.Errorf("Line %d has width %d, expected %d: %q", i, lineWidth, width, line)
		}
	}
}

func TestCreateTableLines_VisualAlignment(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}

	tests := []struct {
		name          string
		alignment     []Alignment
		columnSpacing int
		table         [][]string
		expected      string
	}{
		{
			name:          "Left alignment",
			alignment:     []Alignment{Left, Left, Left},
			columnSpacing: 2,
			table: [][]string{
				{"Name", "Age", "City"},
				{"Alice", "30", "NYC"},
				{"Bob", "25", "LA"},
			},
			expected: `
Name   Age  City|
Alice  30   NYC |
Bob    25   LA  |`,
		},
		{
			name:          "Right alignment",
			alignment:     []Alignment{Right, Right, Right},
			columnSpacing: 2,
			table: [][]string{
				{"Name", "Age", "City"},
				{"Alice", "30", "NYC"},
				{"Bob", "25", "LA"},
			},
			expected: `
 Name  Age  City|
Alice   30   NYC|
  Bob   25    LA|`,
		},
		{
			name:          "Center alignment",
			alignment:     []Alignment{Center, Center, Center},
			columnSpacing: 2,
			table: [][]string{
				{"Name", "Age", "City"},
				{"Alice", "30", "NYC"},
				{"Bob", "25", "LA"},
			},
			expected: `
Name   Age  City|
Alice  30   NYC |
 Bob   25    LA |`,
		},
		{
			name:          "Mixed alignment",
			alignment:     []Alignment{Left, Right, Center},
			columnSpacing: 3,
			table: [][]string{
				{"Product", "Price", "Stock"},
				{"Apple", "1.50", "100"},
				{"Banana", "0.75", "50"},
			},
			expected: `
Product   Price   Stock|
Apple      1.50    100 |
Banana     0.75    50  |`,
		},
		{
			name:          "No spacing",
			alignment:     []Alignment{Left, Center, Right},
			columnSpacing: 0,
			table: [][]string{
				{"A", "BB", "CCC"},
				{"X", "Y", "Z"},
			},
			expected: `
ABBCCC|
XY   Z|`,
		},
		{
			name:          "Wide spacing",
			alignment:     []Alignment{Left, Right},
			columnSpacing: 5,
			table: [][]string{
				{"Foo", "Bar"},
				{"A", "B"},
			},
			expected: `
Foo     Bar|
A         B|`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, _ := CreateTableLines(ap, tt.alignment, tt.columnSpacing, tt.table, BorderNone)
			result := "\n" + strings.Join(lines, "\n")
			expected := strings.ReplaceAll(tt.expected, "|", "")

			if result != expected {
				t.Errorf("\nExpected:\n%s\n\nGot:\n%s", expected, result)
			}
		})
	}
}

func TestWriteTableBoxed(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	ap := &ansipixels.AnsiPixels{
		W:   80,
		H:   24,
		Out: writer,
	}

	alignment := []Alignment{Left, Right, Center}
	columnSpacing := 2
	table := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "NYC"},
		{"Bob", "25", "LA"},
	}

	y := 5
	width := WriteTable(ap, y, alignment, columnSpacing, table, BorderOuter)

	// Flush the writer to get the output
	writer.Flush()

	// Check that width was returned
	if width == 0 {
		t.Error("Expected non-zero width")
	}

	// Check that something was written to the buffer
	output := buf.String()
	if len(output) == 0 {
		t.Error("Expected output to be written to buffer")
	}

	// Check that the output contains the table data
	if !strings.Contains(output, "Name") {
		t.Error("Expected output to contain 'Name'")
	}
	if !strings.Contains(output, "Alice") {
		t.Error("Expected output to contain 'Alice'")
	}
	if !strings.Contains(output, "Bob") {
		t.Error("Expected output to contain 'Bob'")
	}

	// Check that MoveCursor was called (cursor positioning escape codes should be present)
	// ANSI escape codes for cursor movement start with ESC[
	if !strings.Contains(output, "\x1b[") {
		t.Error("Expected output to contain ANSI escape codes for cursor positioning")
	}
}

func TestWriteTable_BorderStyles(t *testing.T) {
	tests := []struct {
		name        string
		borderStyle BorderStyle
	}{
		{"BorderNone", BorderNone},
		{"BorderColumns", BorderColumns},
		{"BorderOuter", BorderOuter},
		{"BorderOuterColumns", BorderOuterColumns},
		{"BorderFull", BorderFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := bufio.NewWriter(&buf)
			ap := &ansipixels.AnsiPixels{
				W:   80,
				H:   24,
				Out: writer,
			}

			alignment := []Alignment{Left, Left}
			columnSpacing := 1
			table := [][]string{
				{"A", "B"},
				{"C", "D"},
			}

			y := 5
			width := WriteTable(ap, y, alignment, columnSpacing, table, tt.borderStyle)
			writer.Flush()

			if width == 0 {
				t.Error("Expected non-zero width")
			}

			output := buf.String()
			if len(output) == 0 {
				t.Error("Expected output to be written to buffer")
			}
		})
	}
}

//nolint:funlen // it's a test.
func TestCreateTableLines_BorderStyles(t *testing.T) {
	ap := &ansipixels.AnsiPixels{}

	tests := []struct {
		name          string
		borderStyle   BorderStyle
		alignment     []Alignment
		columnSpacing int
		table         [][]string
		expected      string
	}{
		{
			name:          "BorderColumns",
			borderStyle:   BorderColumns,
			alignment:     []Alignment{Left, Left, Left},
			columnSpacing: 1,
			table: [][]string{
				{"Name", "Age", "City"},
				{"Alice", "30", "NYC"},
				{"Bob", "25", "LA"},
			},
			expected: `
 Name  │ Age │ City |
 Alice │ 30  │ NYC  |
 Bob   │ 25  │ LA   |`,
		},
		{
			name:          "BorderOuterColumns",
			borderStyle:   BorderOuterColumns,
			alignment:     []Alignment{Left, Left, Left},
			columnSpacing: 1,
			table: [][]string{
				{"Name", "Age", "City"},
				{"Alice", "30", "NYC"},
			},
			expected: `
┌───────┬─────┬──────┐|
│ Name  │ Age │ City │|
│ Alice │ 30  │ NYC  │|
└───────┴─────┴──────┘|`,
		},
		{
			name:          "BorderFull",
			borderStyle:   BorderFull,
			alignment:     []Alignment{Left, Left},
			columnSpacing: 1,
			table: [][]string{
				{"Name", "Age"},
				{"Alice", "30"},
				{"Bob", "25"},
			},
			expected: `
┌───────┬─────┐|
│ Name  │ Age │|
├───────┼─────┤|
│ Alice │ 30  │|
├───────┼─────┤|
│ Bob   │ 25  │|
└───────┴─────┘|`,
		},
		{
			name:          "BorderNone with spacing",
			borderStyle:   BorderNone,
			alignment:     []Alignment{Left, Right},
			columnSpacing: 3,
			table: [][]string{
				{"Foo", "Bar"},
				{"A", "B"},
			},
			expected: `
Foo   Bar|
A       B|`,
		},
		{
			name:          "BorderOuterColumns spacing=0",
			borderStyle:   BorderOuterColumns,
			alignment:     []Alignment{Left, Left},
			columnSpacing: 0,
			table: [][]string{
				{"Name", "Age"},
				{"Alice", "30"},
			},
			expected: `
┌─────┬───┐|
│Name │Age│|
│Alice│30 │|
└─────┴───┘|`,
		},
		{
			name:          "BorderOuterColumns spacing=2",
			borderStyle:   BorderOuterColumns,
			alignment:     []Alignment{Left, Left},
			columnSpacing: 2,
			table: [][]string{
				{"Name", "Age"},
				{"Alice", "30"},
			},
			expected: `
┌─────────┬───────┐|
│  Name   │  Age  │|
│  Alice  │  30   │|
└─────────┴───────┘|`,
		},
		{
			name:          "BorderColumns spacing=0",
			borderStyle:   BorderColumns,
			alignment:     []Alignment{Left, Right},
			columnSpacing: 0,
			table: [][]string{
				{"X", "Y"},
				{"A", "B"},
			},
			expected: `
X│Y|
A│B|`,
		},
		{
			name:          "BorderColumns spacing=1",
			borderStyle:   BorderColumns,
			alignment:     []Alignment{Left, Right},
			columnSpacing: 1,
			table: [][]string{
				{"X", "Y"},
				{"A", "B"},
			},
			expected: `
 X │ Y |
 A │ B |`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, _ := CreateTableLines(ap, tt.alignment, tt.columnSpacing, tt.table, tt.borderStyle)
			result := "\n" + strings.Join(lines, "\n")
			expected := strings.ReplaceAll(tt.expected, "|", "")

			if result != expected {
				t.Errorf("\nExpected:\n%s\n\nGot:\n%s", expected, result)
			}
		})
	}
}
