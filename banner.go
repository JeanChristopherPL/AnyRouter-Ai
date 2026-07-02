package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF"))
	styleGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF87"))
	styleYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	styleMagenta = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6BCB"))
	styleRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	styleOk      = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	styleInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("#55BBFF"))
)

var letterBlocks = map[rune][5]string{
	'A': {"  █████  ",
	      " ██   ██ ",
		  " ███████ ",
		  " ██   ██ ",
		  " ██   ██ "},
	'n': {" ██   ██ ",
	      " ███  ██ ",
		  " ██ █ ██ ",
		  " ██  ███ ",
		  " ██   ██ "},
	'y': {" ██    ██ ",
		  "  ██  ██  ",
		  "   ████   ",
		  "    ██    ",
		  "    ██    "},
	'R': {" ██████  ",
	      " ██   ██ ",
		  " ██████  ",
		  " ██  ██  ",
		  " ██   ██ "},
	'o': {"  █████  ",
	      " ██   ██ ",
		  " ██   ██ ",
		  " ██   ██ ",
		  "  █████  "},
	'u': {" ██   ██ ",
	      " ██   ██ ",
		  " ██   ██ ",
		  " ██   ██ ",
		  "  █████  "},
	't': {" ████████ ",
	      "    ██    ",
		  "    ██    ",
		  "    ██    ",
		  "    ██    "},
	'e': {"  █████   ",
	      " ██   ██  ",
		  " ██████   ",
		  " ██       ",
		  "  █████   "},
	'r': {" ██████    ",
	      " ██   ██   ",
		  " █████     ",
		  " ██   ██   ",
		  " ██    ██  "},
}

func RenderBanner() string {
	return renderBlockBanner("AnyRouter", styleCyan) + "\n" +
		styleDivider.Render("  ─────────────────────────────────────────────────────") + "\n" +
		styleDim.Render("  Universal LLM API Router & Converter  ·  v" + Version) + "\n" +
		styleDivider.Render("  ─────────────────────────────────────────────────────") + "\n"
}

func RenderMiniBanner() string {
	fig := renderBlockBanner("AnyRouter", styleDim)
	lines := strings.Split(fig, "\n")
	if len(lines) > 3 {
		lines = lines[:3]
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderBlockBanner(text string, style lipgloss.Style) string {
	rows := [5]string{}
	for _, ch := range text {
		if block, ok := letterBlocks[ch]; ok {
			for i := 0; i < 5; i++ {
				rows[i] += block[i]
			}
		} else {
			// Skip unknown characters
			continue
		}
	}

	result := ""
	for _, row := range rows {
		result += style.Render(row) + "\n"
	}
	return result
}

func styleStatus(enabled, degraded bool, detail string) string {
	switch {
	case enabled && !degraded:
		return styleOk.Render("●") + " " + detail
	case enabled && degraded:
		return styleWarn.Render("◐") + " " + detail
	default:
		return styleRed.Render("○") + " " + detail
	}
}

func styleServerBadge(port int) string {
	return styleGreen.Render(fmt.Sprintf("  ● Running :%d  ", port))
}

func styleError(msg string) string {
	return styleRed.Render("✗ " + msg)
}
