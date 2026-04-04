package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors — consistent palette across the app
var (
	Purple    = lipgloss.Color("#7C3AED")
	Blue      = lipgloss.Color("#60A5FA")
	Green     = lipgloss.Color("#34D399")
	Yellow    = lipgloss.Color("#FBBF24")
	Red       = lipgloss.Color("#EF4444")
	White     = lipgloss.Color("#F3F4F6")
	Gray      = lipgloss.Color("#9CA3AF")
	LightGray = lipgloss.Color("#E5E7EB")
	DimGray   = lipgloss.Color("#6B7280")
	DarkGray  = lipgloss.Color("#374151")
	DarkBg    = lipgloss.Color("#1F2937")
	Violet    = lipgloss.Color("#A78BFA")
)

// Reusable styles
var (
	Title = lipgloss.NewStyle().
		Foreground(Purple).
		Bold(true)

	Selected = lipgloss.NewStyle().
			Background(Purple).
			Foreground(White).
			Bold(true)

	Normal = lipgloss.NewStyle().
		Foreground(LightGray)

	Dim = lipgloss.NewStyle().
		Foreground(DimGray)

	Error = lipgloss.NewStyle().
		Foreground(Red).
		Bold(true)

	Success = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)

	Warning = lipgloss.NewStyle().
			Foreground(Yellow).
			Bold(true)

	UserMsg = lipgloss.NewStyle().
		Foreground(Blue).
		Bold(true)

	AssistantMsg = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)

	CodeBlock = lipgloss.NewStyle().
			Foreground(Violet).
			Background(DarkBg).
			Padding(0, 1)

	KeyStyle = lipgloss.NewStyle().
			Foreground(Green)

	Separator = lipgloss.NewStyle().
			Foreground(DimGray)

	DiffAdd = lipgloss.NewStyle().
			Foreground(Green)

	DiffRemove = lipgloss.NewStyle().
			Foreground(Red)
)

// Footer builds a consistent footer from key-description pairs.
// Example: Footer("enter", "select", "esc", "back")
func Footer(pairs ...string) string {
	var b strings.Builder
	sep := Separator.Render(" | ")
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(KeyStyle.Render(pairs[i]))
		b.WriteString(Dim.Render(": " + pairs[i+1]))
	}
	return b.String()
}
