package pgn

import (
	"strings"
	"testing"
)

const opera = `[Event "Paris Opera"]
[Site "Paris"]
[Date "1858.??.??"]
[White "Paul Morphy"]
[Black "Duke Karl / Count Isouard"]
[Result "1-0"]

{A model of quick development.} 1. e4 e5 2. Nf3 d6 3. d4 Bg4?! {Better is}
(3... exd4 4. Nxd4 (4. Qxd4 Nc6) Nf6) 4. dxe5 Bxf3 5. Qxf3 dxe5 6. Bc4 Nf6
7. Qb3 Qe7 8. Nc3 c6 9. Bg5 b5? 10. Nxb5! cxb5 11. Bxb5+ Nbd7 12. O-O-O Rd8
13. Rxd7 Rxd7 14. Rd1 Qe6 15. Bxd7+ Nxd7 16. Qb8+ $1 Nxb8 17. Rd8# 1-0

[Event "Second"]
[Result "*"]

1.d4 d5 2.c4 *
`

func TestParseKeepsCommentsVariationsAndNAGs(t *testing.T) {
	games, err := Parse(opera)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 2 {
		t.Fatalf("games = %d", len(games))
	}
	game := games[0]
	if game.Err != nil {
		t.Fatal(game.Err)
	}
	main := game.Mainline()
	if len(main) != 33 || main[len(main)-1].SAN != "Rd8#" || game.Result != "1-0" {
		t.Fatalf("mainline %d, last %s, result %s", len(main), main[len(main)-1].SAN, game.Result)
	}
	if game.Root.Comment != "A model of quick development." {
		t.Fatalf("game comment %q", game.Root.Comment)
	}
	bg4 := main[5]
	if bg4.SAN != "Bg4" || len(bg4.NAGs) != 1 || bg4.NAGs[0] != 6 || bg4.Comment != "Better is" {
		t.Fatalf("Bg4 = %+v", bg4)
	}
	if len(bg4.Parent.Children) != 2 || bg4.Parent.Children[1].SAN != "exd4" {
		t.Fatal("side line 3...exd4 missing")
	}
	nxd4 := bg4.Parent.Children[1].Children[0]
	if len(nxd4.Parent.Children) != 2 || nxd4.Parent.Children[1].SAN != "Qxd4" {
		t.Fatal("nested side line 4.Qxd4 missing")
	}
	if games[1].Title() != "Second" || len(games[1].Mainline()) != 3 {
		t.Fatalf("second game = %s, %d moves", games[1].Title(), len(games[1].Mainline()))
	}
}

func TestWriteRoundTrips(t *testing.T) {
	games, _ := Parse(opera)
	text := games[0].String()
	for _, want := range []string{`[White "Paul Morphy"]`, "{A model of quick development.}", "3. d4 Bg4?! {Better is} (3... exd4 4. Nxd4 (4. Qxd4 Nc6) 4... Nf6) 4. dxe5", "16. Qb8+! Nxb8", "17. Rd8# 1-0"} {
		if !strings.Contains(strings.ReplaceAll(text, "\n", " "), want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	again, err := Parse(text)
	if err != nil || again[0].String() != text {
		t.Fatalf("second round differs:\n%s", again[0].String())
	}
}

func TestPlayBranchesAndPromotes(t *testing.T) {
	game, _ := New("")
	e4, _ := game.Root.Play("e4")
	d4, _ := game.Root.Play("d2d4")
	if again, _ := game.Root.Play("e2e4"); again != e4 {
		t.Fatal("playing an existing move should reuse it")
	}
	d4.Promote()
	if game.Root.Children[0] != d4 {
		t.Fatal("promote failed")
	}
	e4.Delete()
	if len(game.Root.Children) != 1 {
		t.Fatal("delete failed")
	}
	if _, err := d4.Play("e4e5"); err == nil {
		t.Fatal("illegal move accepted")
	}
	if line := d4.Line(); len(line) != 1 || line[0] != "d2d4" {
		t.Fatalf("line = %v", line)
	}
}

func TestIllegalMoveKeepsWhatCameBefore(t *testing.T) {
	games, err := Parse("1. e4 e5 2. Ke3 Nc6 *")
	if err != nil || games[0].Err == nil || len(games[0].Mainline()) != 2 {
		t.Fatalf("err=%v gameErr=%v moves=%d", err, games[0].Err, len(games[0].Mainline()))
	}
}
