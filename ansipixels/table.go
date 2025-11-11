// Package table provides functions to render tables in terminal using ansipixels.
// TODO: move to fortio.org/terminal/ansipixels/table
package table

import (
	"strings"

	"fortio.org/terminal/ansipixels"
)

type Alignment int

const (
	Left Alignment = iota
	Center
	Right
)

type BorderStyle int

const (
	BorderNone         BorderStyle = iota // No borders at all
	BorderColumns                         // Only vertical lines between columns (│)
	BorderOuter                           // Only outer box around the table
	BorderOuterColumns                    // Outer box + column separators
	BorderFull                            // Full grid with all cell borders
)

// WriteTable renders a table at the specified y position with the given border style.
// The table is centered horizontally on the screen.
// Returns the width of the table content (excluding borders).
func WriteTable(
	ap *ansipixels.AnsiPixels, y int, alignment []Alignment,
	columnSpacing int, table [][]string, borderStyle BorderStyle,
) int {
	lines, width := CreateTableLines(ap, alignment, columnSpacing, table, borderStyle)
	var cursorY int
	leftX := (ap.W - width) / 2
	for i, l := range lines {
		cursorY = y + i
		ap.MoveCursor(leftX, cursorY)
		ap.WriteString(l)
	}
	switch borderStyle {
	case BorderOuter:
		// Only BorderOuter needs an additional round box, as the table lines don't include borders
		ap.DrawRoundBox((ap.W-width)/2-1, y-1, width+2, len(lines)+2)
	case BorderNone, BorderColumns, BorderOuterColumns, BorderFull:
		// These styles either have no borders or already drew them in CreateTableLines
	}
	return width
}

// drawHorizontalBorder creates a horizontal border line with the specified corner/junction characters.
func drawHorizontalBorder(ncols int, colWidths []int, columnSpacing int, left, middle, right string) string {
	var sb strings.Builder
	sb.WriteString(left)
	for j := range ncols {
		sb.WriteString(strings.Repeat(ansipixels.Horizontal, colWidths[j]+2*columnSpacing))
		if j < ncols-1 {
			sb.WriteString(middle)
		}
	}
	sb.WriteString(right)
	return sb.String()
}

// calculateColumnWidths computes the maximum width needed for each column
// and returns both the column widths and all individual cell widths.
func calculateColumnWidths(ap *ansipixels.AnsiPixels, table [][]string, ncols int) ([]int, [][]int) {
	nrows := len(table)
	colWidths := make([]int, ncols)
	allWidths := make([][]int, 0, nrows)
	for _, row := range table {
		if len(row) != ncols {
			panic("inconsistent number of columns in table")
		}
		allWidthsRow := make([]int, 0, ncols)
		for j, cell := range row {
			w := ap.ScreenWidth(cell)
			allWidthsRow = append(allWidthsRow, w)
			if w > colWidths[j] {
				colWidths[j] = w
			}
		}
		allWidths = append(allWidths, allWidthsRow)
	}
	return colWidths, allWidths
}

// calculateTableWidth computes the total width of the table including borders and spacing.
func calculateTableWidth(colWidths []int, ncols, columnSpacing int, hasColumnBorders, hasOuterBorder bool) int {
	maxw := 0
	for _, w := range colWidths {
		maxw += w
		if hasColumnBorders {
			maxw += 2 * columnSpacing
		}
	}
	// Add spacing/separators between columns
	if ncols > 1 {
		if hasColumnBorders {
			maxw += (ncols - 1) // vertical separators between columns
		} else {
			maxw += columnSpacing * (ncols - 1)
		}
	}
	// Add outer borders if present
	if hasOuterBorder {
		maxw += 2 // left and right borders
	}
	return maxw
}

// formatCell formats a single cell with the specified alignment and padding.
func formatCell(sb *strings.Builder, cell string, cellWidth, columnWidth, columnSpacing int,
	align Alignment, hasColumnBorders bool,
) {
	delta := columnWidth - cellWidth

	// Add padding before content
	if hasColumnBorders {
		sb.WriteString(strings.Repeat(" ", columnSpacing))
	}

	// Add aligned content
	switch align {
	case Left:
		sb.WriteString(cell)
		sb.WriteString(strings.Repeat(" ", delta))
	case Center:
		sb.WriteString(strings.Repeat(" ", delta/2))
		sb.WriteString(cell)
		sb.WriteString(strings.Repeat(" ", delta/2+delta%2))
	case Right:
		sb.WriteString(strings.Repeat(" ", delta))
		sb.WriteString(cell)
	}

	// Add padding after content
	if hasColumnBorders {
		sb.WriteString(strings.Repeat(" ", columnSpacing))
	}
}

func CreateTableLines(ap *ansipixels.AnsiPixels,
	alignment []Alignment,
	columnSpacing int,
	table [][]string,
	borderStyle BorderStyle,
) ([]string, int) {
	nrows := len(table)
	ncols := len(alignment)

	// Calculate column widths
	colWidths, allWidths := calculateColumnWidths(ap, table, ncols)

	// Determine spacing between columns based on border style
	hasColumnBorders := borderStyle == BorderColumns || borderStyle == BorderOuterColumns || borderStyle == BorderFull
	hasOuterBorder := borderStyle == BorderOuterColumns || borderStyle == BorderFull

	// Calculate total width
	maxw := calculateTableWidth(colWidths, ncols, columnSpacing, hasColumnBorders, hasOuterBorder)

	// Calculate exact number of lines needed
	numLines := nrows
	if borderStyle == BorderFull {
		numLines = 2*nrows + 1 // data rows + row separators + top/bottom borders
	} else if hasOuterBorder {
		numLines = nrows + 2 // data rows + top/bottom borders
	}

	// Build table lines using direct indexing to catch capacity errors
	lines := make([]string, numLines)
	var sb strings.Builder
	lineIdx := 0

	// Add top border if needed
	if hasOuterBorder {
		lines[lineIdx] = drawHorizontalBorder(ncols, colWidths, columnSpacing,
			ansipixels.SquareTopLeft, ansipixels.TopT, ansipixels.SquareTopRight)
		lineIdx++
	}

	// Add data rows
	for i, row := range table {
		rowWidth := allWidths[i]

		// Add row separator for full borders (except before first row)
		if borderStyle == BorderFull && i > 0 {
			lines[lineIdx] = drawHorizontalBorder(ncols, colWidths, columnSpacing,
				ansipixels.LeftT, ansipixels.MiddleCross, ansipixels.RightT)
			lineIdx++
		}

		// Add left border if needed
		if hasOuterBorder {
			sb.WriteString(ansipixels.Vertical)
		}

		// Build the data row
		for j, cell := range row {
			formatCell(&sb, cell, rowWidth[j], colWidths[j], columnSpacing, alignment[j], hasColumnBorders)

			// Add column separator or spacing
			if j < ncols-1 {
				separator := strings.Repeat(" ", columnSpacing)
				if hasColumnBorders {
					separator = ansipixels.Vertical
				}
				sb.WriteString(separator)
			}
		}

		// Add right border if needed
		if hasOuterBorder {
			sb.WriteString(ansipixels.Vertical)
		}

		lines[lineIdx] = sb.String()
		lineIdx++
		sb.Reset()
	}

	// Add bottom border if needed
	if hasOuterBorder {
		lines[lineIdx] = drawHorizontalBorder(ncols, colWidths, columnSpacing,
			ansipixels.SquareBottomLeft, ansipixels.BottomT, ansipixels.SquareBottomRight)
	}

	return lines, maxw
}
