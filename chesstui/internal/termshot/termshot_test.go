package termshot

import (
	"strings"
	"testing"
)

func TestSVGKeepsColoursAndEscapesText(t *testing.T) {
	frame := "\x1b[1;38;2;226;181;92m♞\x1b[0m <b>\n\x1b[48;2;169;180;151m  \x1b[m█"
	svg := SVG(frame, Options{Background: "#111111", Foreground: "#eeeeee"})
	for _, want := range []string{`fill="#e2b55c"`, `font-weight="700"`, "♞", "&lt;b&gt;", `fill="#a9b497"`} {
		if !strings.Contains(svg, want) {
			t.Fatalf("SVG missing %s:\n%s", want, svg)
		}
	}
	if strings.Contains(svg, "\x1b") {
		t.Fatal("escape sequences leaked into the SVG")
	}
}
