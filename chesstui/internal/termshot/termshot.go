// Package termshot turns a rendered ANSI frame into an SVG image, so
// screenshots in the docs are generated from the real UI rather than drawn.
package termshot

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Options controls the frame drawn around the terminal grid.
type Options struct {
	Background string // terminal background, e.g. "#111214"
	Foreground string // default text colour
	Title      string // optional window title
	FontSize   float64
}

type cell struct {
	ch        string
	fg, bg    string
	bold      bool
	faint     bool
	underline bool
}

type style struct {
	fg, bg          string
	bold, faint, ul bool
}

// Parse splits frame into rows of cells, applying SGR colour sequences.
func parse(frame string) [][]cell {
	rows := [][]cell{}
	for _, line := range strings.Split(frame, "\n") {
		row := []cell{}
		var current style
		for i := 0; i < len(line); {
			if line[i] == 0x1b {
				end := i + 1
				if end < len(line) && line[end] == '[' {
					end++
					for end < len(line) && (line[end] < 0x40 || line[end] > 0x7e) {
						end++
					}
					if end < len(line) && line[end] == 'm' {
						applySGR(&current, line[i+2:end])
					}
					i = end + 1
					continue
				}
				// OSC or other escape: skip to its terminator.
				if end < len(line) && line[end] == ']' {
					for end < len(line) && line[end] != 0x07 && !(line[end] == 0x1b && end+1 < len(line) && line[end+1] == '\\') {
						end++
					}
					i = end + 1
					continue
				}
				i = end + 1
				continue
			}
			grapheme, width := ansi.FirstGraphemeCluster(line[i:], ansi.GraphemeWidth)
			if grapheme == "" {
				break
			}
			c := cell{ch: grapheme, fg: current.fg, bg: current.bg, bold: current.bold, faint: current.faint, underline: current.ul}
			row = append(row, c)
			for range width - 1 {
				row = append(row, cell{ch: "", fg: current.fg, bg: current.bg})
			}
			i += len(grapheme)
		}
		rows = append(rows, row)
	}
	return rows
}

func applySGR(s *style, params string) {
	if params == "" {
		*s = style{}
		return
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		switch {
		case n == 0:
			*s = style{}
		case n == 1:
			s.bold = true
		case n == 2:
			s.faint = true
		case n == 4:
			s.ul = true
		case n == 22:
			s.bold, s.faint = false, false
		case n == 24:
			s.ul = false
		case n == 39:
			s.fg = ""
		case n == 49:
			s.bg = ""
		case (n == 38 || n == 48) && i+4 < len(parts) && parts[i+1] == "2":
			r, _ := strconv.Atoi(parts[i+2])
			g, _ := strconv.Atoi(parts[i+3])
			b, _ := strconv.Atoi(parts[i+4])
			color := fmt.Sprintf("#%02x%02x%02x", r, g, b)
			if n == 38 {
				s.fg = color
			} else {
				s.bg = color
			}
			i += 4
		case (n == 38 || n == 48) && i+2 < len(parts) && parts[i+1] == "5":
			idx, _ := strconv.Atoi(parts[i+2])
			if n == 38 {
				s.fg = xterm(idx)
			} else {
				s.bg = xterm(idx)
			}
			i += 2
		case n >= 30 && n <= 37:
			s.fg = xterm(n - 30)
		case n >= 90 && n <= 97:
			s.fg = xterm(n - 90 + 8)
		case n >= 40 && n <= 47:
			s.bg = xterm(n - 40)
		}
	}
}

func xterm(i int) string {
	basic := []string{"#000000", "#cd3131", "#0dbc79", "#e5e510", "#2472c8", "#bc3fbc", "#11a8cd", "#e5e5e5",
		"#666666", "#f14c4c", "#23d18b", "#f5f543", "#3b8eea", "#d670d6", "#29b8db", "#ffffff"}
	if i >= 0 && i < 16 {
		return basic[i]
	}
	return "#888888"
}

// SVG renders frame as a standalone SVG document.
func SVG(frame string, opts Options) string {
	if opts.FontSize == 0 {
		opts.FontSize = 14
	}
	rows := parse(frame)
	cols := 0
	for _, row := range rows {
		cols = max(cols, len(row))
	}
	cw, lh := opts.FontSize*0.6, opts.FontSize*1.3
	pad := opts.FontSize * 1.4
	top := pad
	if opts.Title != "" {
		top += opts.FontSize * 1.6
	}
	width := float64(cols)*cw + pad*2
	height := float64(len(rows))*lh + top + pad

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f" font-family="'JetBrains Mono','SF Mono',Menlo,Consolas,'DejaVu Sans Mono',monospace" font-size="%.0f">`+"\n",
		width, height, width, height, opts.FontSize)
	fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" rx="12" fill="%s"/>`+"\n", width, height, opts.Background)
	if opts.Title != "" {
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="%s" fill-opacity="0.45" text-anchor="middle" font-size="%.0f">%s</text>`+"\n",
			width/2, pad*0.9, opts.Foreground, opts.FontSize*0.85, html.EscapeString(opts.Title))
	}
	b.WriteString(`<g text-rendering="geometricPrecision">` + "\n")
	for y, row := range rows {
		baseY := top + float64(y)*lh
		// Backgrounds as merged runs.
		for x := 0; x < len(row); {
			bg := row[x].bg
			end := x + 1
			for end < len(row) && row[end].bg == bg {
				end++
			}
			if bg != "" {
				fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="%s"/>`, pad+float64(x)*cw, baseY, float64(end-x)*cw+0.3, lh+0.3, bg)
			}
			x = end
		}
		// Text: ASCII runs share a <text> with a fixed advance; other glyphs
		// are centred in their own cell so symbol fallback fonts line up.
		for x := 0; x < len(row); {
			c := row[x]
			if strings.TrimSpace(c.ch) == "" {
				x++
				continue
			}
			fg := c.fg
			if fg == "" {
				fg = opts.Foreground
			}
			// Block elements become rectangles so they tile without the
			// gaps a font's glyph box would leave between rows.
			if box, ok := blocks[c.ch]; ok {
				fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="%s"/>`,
					pad+(float64(x)+box[0])*cw, baseY+box[1]*lh, box[2]*cw+0.6, box[3]*lh+0.6, fg)
				x++
				continue
			}
			ascii := len(c.ch) == 1 && c.ch[0] < 0x80
			end := x + 1
			if ascii {
				for end < len(row) && len(row[end].ch) == 1 && row[end].ch[0] < 0x80 && sameText(row[end], c) {
					end++
				}
			}
			text := strings.Builder{}
			for _, part := range row[x:end] {
				text.WriteString(part.ch)
			}
			attrs := fmt.Sprintf(`fill="%s"`, fg)
			if c.bold {
				attrs += ` font-weight="700"`
			}
			if c.faint {
				attrs += ` fill-opacity="0.6"`
			}
			if c.underline {
				attrs += ` text-decoration="underline"`
			}
			textY := baseY + lh*0.76
			if ascii {
				fmt.Fprintf(&b, `<text x="%.2f" y="%.2f" %s xml:space="preserve" textLength="%.2f" lengthAdjust="spacingAndGlyphs">%s</text>`,
					pad+float64(x)*cw, textY, attrs, float64(end-x)*cw, html.EscapeString(text.String()))
			} else {
				fmt.Fprintf(&b, `<text x="%.2f" y="%.2f" %s text-anchor="middle">%s</text>`,
					pad+(float64(x)+0.5)*cw, textY, attrs, html.EscapeString(text.String()))
			}
			x = end
		}
		b.WriteString("\n")
	}
	b.WriteString("</g>\n</svg>\n")
	return b.String()
}

// blocks maps block characters to {x, y, w, h} as fractions of a cell.
var blocks = map[string][4]float64{
	"█": {0, 0, 1, 1},
	"▀": {0, 0, 1, 0.5},
	"▄": {0, 0.5, 1, 0.5},
	"▌": {0, 0, 0.5, 1},
	"▐": {0.5, 0, 0.5, 1},
	"▏": {0.05, 0.15, 0.12, 0.7},
}

func sameText(a, b cell) bool {
	return a.fg == b.fg && a.bold == b.bold && a.faint == b.faint && a.underline == b.underline
}
