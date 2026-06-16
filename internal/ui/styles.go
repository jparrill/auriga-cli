package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Tokyo Night palette
var (
	ColorBg      = lipgloss.Color("#1a1b26")
	ColorSurface = lipgloss.Color("#24283b")
	ColorText    = lipgloss.Color("#c0caf5")
	ColorMuted   = lipgloss.Color("#565f89")
	ColorAccent  = lipgloss.Color("#7aa2f7")
	ColorGreen   = lipgloss.Color("#9ece6a")
	ColorYellow  = lipgloss.Color("#e0af68")
	ColorRed     = lipgloss.Color("#f7768e")
	ColorPurple  = lipgloss.Color("#bb9af7")
	ColorCyan    = lipgloss.Color("#7dcfff")
)

var (
	SuccessStyle = lipgloss.NewStyle().Foreground(ColorGreen)
	ErrorStyle   = lipgloss.NewStyle().Foreground(ColorRed)
	WarningStyle = lipgloss.NewStyle().Foreground(ColorYellow)
	InfoStyle    = lipgloss.NewStyle().Foreground(ColorCyan)
	AccentStyle  = lipgloss.NewStyle().Foreground(ColorAccent)
	MutedStyle   = lipgloss.NewStyle().Foreground(ColorMuted)
	BoldStyle    = lipgloss.NewStyle().Bold(true)

	KeyStyle   = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	ValueStyle = lipgloss.NewStyle().Foreground(ColorText)

	SummaryBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorAccent).
			Padding(1, 2)

	DestructiveBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorRed).
			Padding(1, 2)
)

func Ok(msg string)   { fmt.Printf("  %s %s\n", SuccessStyle.Render("✓"), msg) }
func Fail(msg string)  { fmt.Printf("  %s %s\n", ErrorStyle.Render("✗"), msg) }
func Warn(msg string)  { fmt.Printf("  %s %s\n", WarningStyle.Render("⚠"), msg) }
func Info(msg string)  { fmt.Printf("  %s %s\n", InfoStyle.Render("→"), msg) }

func FormatKeyValue(key, value string) string {
	return fmt.Sprintf("%s %s", KeyStyle.Render(key+":"), ValueStyle.Render(value))
}
