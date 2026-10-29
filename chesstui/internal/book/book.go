// Package book reads chess books for the terminal reader. Two formats are
// supported: annotated PGN, where every game is a chapter and comments are
// the prose, and Markdown with fenced ```pgn and ```fen blocks.
package book

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kannachi323/chesstui/internal/pgn"
)

// Kind is the type of a block of book content.
type Kind int

const (
	Heading Kind = iota
	Paragraph
	Quote
	Item
	Diagram
	Game
)

type Block struct {
	Kind    Kind
	Text    string    // heading, paragraph, quote or list item; diagram caption
	FEN     string    // diagram
	Game    *pgn.Game // game
	Comment bool      // a paragraph that came from a PGN comment
}

type Chapter struct {
	Title  string
	Blocks []Block
}

type Book struct {
	Title    string
	Author   string
	Chapters []Chapter
}

//go:embed starter.md
var starter string

// StarterID identifies the book built into chesstui.
const StarterID = "builtin:starter"

// Starter returns the built-in introductory book.
func Starter() *Book {
	b, err := ParseMarkdown(starter)
	if err != nil {
		panic(err) // covered by tests; the text is compiled in
	}
	return b
}

// Load reads a .pgn or .md/.markdown/.txt book from disk.
func Load(path string) (*Book, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open book: %w", err)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pgn":
		b, err := ParsePGN(string(data))
		if err == nil && b.Title == "" {
			b.Title = name
		}
		return b, err
	case ".md", ".markdown", ".txt":
		b, err := ParseMarkdown(string(data))
		if err == nil && b.Title == "" {
			b.Title = name
		}
		return b, err
	}
	return nil, fmt.Errorf("%s: books must be .pgn or .md files", filepath.Base(path))
}

// ParsePGN turns each game of an annotated PGN into a chapter. Lichess
// study exports, where each chapter is a game, read naturally this way.
func ParsePGN(text string) (*Book, error) {
	games, err := pgn.Parse(text)
	if err != nil {
		return nil, err
	}
	b := &Book{}
	for i, game := range games {
		title := game.Tags["ChapterName"]
		if title == "" {
			title = game.Title()
		}
		if b.Title == "" {
			if study := game.Tags["StudyName"]; study != "" {
				b.Title = study
			}
		}
		if title == "Untitled game" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		chapter := Chapter{Title: title}
		if game.Root.Comment != "" {
			chapter.Blocks = append(chapter.Blocks, Block{Kind: Paragraph, Text: game.Root.Comment, Comment: true})
		}
		if len(game.Root.Children) > 0 {
			chapter.Blocks = append(chapter.Blocks, Block{Kind: Game, Game: game})
		} else {
			chapter.Blocks = append(chapter.Blocks, Block{Kind: Diagram, FEN: game.Root.FEN})
		}
		b.Chapters = append(b.Chapters, chapter)
	}
	return b, nil
}

// ParseMarkdown reads the Markdown book format:
//
//	# Book title          (first line; "by …" on the next line sets the author)
//	## Chapter title
//	### Section heading
//	paragraphs, > quotes, - list items
//	```fen                a diagram: FEN on the first line, caption after
//	```pgn                a game; its comments and side lines are read inline
func ParseMarkdown(text string) (*Book, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	b := &Book{}
	var chapter *Chapter
	var paragraph []string
	ensure := func() {
		if chapter == nil {
			b.Chapters = append(b.Chapters, Chapter{Title: "Introduction"})
			chapter = &b.Chapters[len(b.Chapters)-1]
		}
	}
	flush := func() {
		if len(paragraph) == 0 {
			return
		}
		ensure()
		joined := strings.Join(paragraph, " ")
		kind := Paragraph
		if strings.HasPrefix(paragraph[0], ">") {
			kind = Quote
			joined = strings.TrimSpace(strings.ReplaceAll(joined, "> ", ""))
			joined = strings.TrimPrefix(joined, ">")
		}
		chapter.Blocks = append(chapter.Blocks, Block{Kind: kind, Text: strings.TrimSpace(joined)})
		paragraph = nil
	}
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			flush()
			lang := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
			var body []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				body = append(body, lines[i])
			}
			ensure()
			switch lang {
			case "pgn":
				games, err := pgn.Parse(strings.Join(body, "\n"))
				if err != nil {
					return nil, fmt.Errorf("chapter %q: %w", chapter.Title, err)
				}
				for _, game := range games {
					if game.Err != nil {
						return nil, fmt.Errorf("chapter %q: %w", chapter.Title, game.Err)
					}
					if game.Root.Comment != "" {
						chapter.Blocks = append(chapter.Blocks, Block{Kind: Paragraph, Text: game.Root.Comment, Comment: true})
					}
					chapter.Blocks = append(chapter.Blocks, Block{Kind: Game, Game: game})
				}
			case "fen":
				if len(body) == 0 {
					continue
				}
				fen := strings.TrimSpace(body[0])
				if _, err := pgn.New(fen); err != nil {
					return nil, fmt.Errorf("chapter %q: %w", chapter.Title, err)
				}
				caption := strings.TrimSpace(strings.Join(body[1:], " "))
				chapter.Blocks = append(chapter.Blocks, Block{Kind: Diagram, FEN: fen, Text: caption})
			default:
				chapter.Blocks = append(chapter.Blocks, Block{Kind: Quote, Text: strings.Join(body, " ")})
			}
		case strings.HasPrefix(trimmed, "### "):
			flush()
			ensure()
			chapter.Blocks = append(chapter.Blocks, Block{Kind: Heading, Text: strings.TrimPrefix(trimmed, "### ")})
		case strings.HasPrefix(trimmed, "## "):
			flush()
			b.Chapters = append(b.Chapters, Chapter{Title: strings.TrimPrefix(trimmed, "## ")})
			chapter = &b.Chapters[len(b.Chapters)-1]
		case strings.HasPrefix(trimmed, "# ") && b.Title == "" && chapter == nil:
			flush()
			b.Title = strings.TrimPrefix(trimmed, "# ")
			if i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "by ") {
				i++
				b.Author = strings.TrimPrefix(strings.TrimSpace(lines[i]), "by ")
			}
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			flush()
			ensure()
			chapter.Blocks = append(chapter.Blocks, Block{Kind: Item, Text: strings.TrimSpace(trimmed[2:])})
		case trimmed == "":
			flush()
		default:
			paragraph = append(paragraph, trimmed)
		}
	}
	flush()
	if len(b.Chapters) == 0 {
		return nil, fmt.Errorf("the book is empty")
	}
	return b, nil
}
