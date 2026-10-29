package book

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStarterBookParses(t *testing.T) {
	b := Starter()
	if b.Title != "First Moves" || b.Author != "chesstui" || len(b.Chapters) != 5 {
		t.Fatalf("title=%q author=%q chapters=%d", b.Title, b.Author, len(b.Chapters))
	}
	games, diagrams := 0, 0
	for _, chapter := range b.Chapters {
		for _, block := range chapter.Blocks {
			switch block.Kind {
			case Game:
				games++
				if block.Game.Err != nil {
					t.Fatalf("%s: %v", chapter.Title, block.Game.Err)
				}
			case Diagram:
				diagrams++
			}
		}
	}
	if games != 3 || diagrams != 2 {
		t.Fatalf("games=%d diagrams=%d", games, diagrams)
	}
}

func TestPGNBookHasAChapterPerGame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "study.pgn")
	text := `[Event "Study: Chapter 1"]
[StudyName "Endgames"]
[ChapterName "Lucena"]
[FEN "1K1k4/1P6/8/8/8/8/r7/2R5 w - - 0 1"]
[SetUp "1"]

{Build a bridge.} 1. Rd1+ Ke7 2. Kc7 *

[ChapterName "Philidor"]

{Third-rank defence.} *
`
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "Endgames" || len(b.Chapters) != 2 || b.Chapters[0].Title != "Lucena" {
		t.Fatalf("book = %+v", b)
	}
	if b.Chapters[0].Blocks[0].Text != "Build a bridge." || b.Chapters[0].Blocks[1].Kind != Game {
		t.Fatalf("chapter 1 blocks = %+v", b.Chapters[0].Blocks)
	}
	if b.Chapters[1].Blocks[1].Kind != Diagram {
		t.Fatal("a game without moves should read as a diagram")
	}
}

func TestBadBooksAreRejected(t *testing.T) {
	if _, err := ParseMarkdown("## Bad\n\n```pgn\n1. e4 e5 2. Ke3 *\n```\n"); err == nil {
		t.Fatal("an illegal move should be reported")
	}
	if _, err := Load(filepath.Join(t.TempDir(), "book.pdf")); err == nil {
		t.Fatal("PDF should be refused")
	}
}
