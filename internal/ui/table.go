package ui

import (
	"fmt"
	"strings"
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
		cols[i] = Column{Header: h, Width: len(h)}
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
		// Only count visible length (strip ANSI)
		visible := stripANSI(v)
		if len(visible) > t.Columns[i].Width {
			t.Columns[i].Width = len(visible)
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
		fmt.Printf("%-*s", col.Width, col.Header)
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
			visible := stripANSI(val)
			padding := col.Width - len(visible)
			if padding < 0 {
				padding = 0
			}
			fmt.Print(val + strings.Repeat(" ", padding))
		}
		fmt.Println()
	}
	fmt.Println()
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
