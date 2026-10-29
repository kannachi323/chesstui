package app

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// fitViewport is the final guardrail that keeps the statusline on the last row
// even when a child screen receives unexpectedly large content.
func fitViewport(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for index := range lines {
		lines[index] = ansi.Truncate(lines[index], width, "…")
	}
	return strings.Join(lines, "\n")
}
