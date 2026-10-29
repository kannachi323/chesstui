package config

import "testing"

func TestGlyphProfileNormalization(t *testing.T) {
	if got := Default().Glyphs; got != "solid" {
		t.Fatalf("default glyph profile = %q", got)
	}
	legacy := Config{Glyphs: "unicode"}
	normalize(&legacy)
	if legacy.Glyphs != "solid" {
		t.Fatalf("legacy unicode profile normalized to %q", legacy.Glyphs)
	}
}
