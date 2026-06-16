package ui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

type Column struct {
	Header string
	Width  int
}

type Table struct {
	Title   string
	Columns []Column
	Rows    [][]string
}

func NewTable(title string, headers ...string) *Table {
	cols := make([]Column, len(headers))
	for i, h := range headers {
		cols[i] = Column{Header: h, Width: runewidth.StringWidth(h)}
	}
	return &Table{Title: title, Columns: cols}
}

func (t *Table) AddRow(values ...string) {
	row := make([]string, len(t.Columns))
	for i, v := range values {
		if i >= len(t.Columns) {
			break
		}
		row[i] = v
		w := visibleWidth(v)
		if w > t.Columns[i].Width {
			t.Columns[i].Width = w
		}
	}
	t.Rows = append(t.Rows, row)
}

func (t *Table) Print() {
	if t.Title != "" {
		fmt.Printf("\n  %s\n", BoldStyle.Render(t.Title))
	}

	totalWidth := 0
	// Print header
	fmt.Print("  ")
	for i, col := range t.Columns {
		if i > 0 {
			fmt.Print("  ")
			totalWidth += 2
		}
		hPad := col.Width - runewidth.StringWidth(col.Header)
		if hPad < 0 {
			hPad = 0
		}
		fmt.Print(col.Header + strings.Repeat(" ", hPad))
		totalWidth += col.Width
	}
	fmt.Println()
	fmt.Printf("  %s\n", strings.Repeat("─", totalWidth))

	// Print rows
	for _, row := range t.Rows {
		fmt.Print("  ")
		for i, col := range t.Columns {
			if i > 0 {
				fmt.Print("  ")
			}
			val := row[i]
			padding := col.Width - visibleWidth(val)
			if padding < 0 {
				padding = 0
			}
			fmt.Print(val + strings.Repeat(" ", padding))
		}
		fmt.Println()
	}
	fmt.Println()
}

func visibleWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

func stripANSI(s string) string {
	var result []byte
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		result = append(result, s[i])
	}
	return string(result)
}
