// Package pgn reads and writes PGN with everything a study needs: comments,
// side lines (variations) and annotation glyphs. Moves are validated as they
// are read, so every node knows its position.
package pgn

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	chesscore "github.com/kannachi323/chesstui/internal/chess"
)

// Node is one move in a game tree. Children[0] continues the line the node
// belongs to; further children are side lines.
type Node struct {
	Parent   *Node
	Children []*Node
	SAN      string
	UCI      string
	NAGs     []int
	Pre      string // comment before the move, at the start of a line
	Comment  string // comment after the move
	FEN      string // position after the move (the root holds the start)
	Ply      int
}

// Game is one PGN game. Root carries no move; its Comment is the game's
// opening remark.
type Game struct {
	Tags     map[string]string
	TagOrder []string
	Root     *Node
	Result   string
	Err      error // set when the movetext held an illegal move; moves up to it are kept
}

var sevenTags = []string{"Event", "Site", "Date", "Round", "White", "Black", "Result"}

// New returns an empty game from the standard start or a FEN.
func New(fen string) (*Game, error) {
	position, err := chesscore.NewPosition(fen)
	if err != nil {
		return nil, err
	}
	game := &Game{Tags: map[string]string{}, Root: &Node{FEN: position.FEN()}, Result: "*"}
	if fen != "" && fen != "startpos" {
		game.SetTag("SetUp", "1")
		game.SetTag("FEN", position.FEN())
	}
	return game, nil
}

func (g *Game) SetTag(key, value string) {
	if _, ok := g.Tags[key]; !ok {
		g.TagOrder = append(g.TagOrder, key)
	}
	g.Tags[key] = value
}

// StartFEN is the game's initial position.
func (g *Game) StartFEN() string { return g.Root.FEN }

// Mainline returns the main line's moves in order.
func (g *Game) Mainline() []*Node {
	var nodes []*Node
	for node := g.Root; len(node.Children) > 0; {
		node = node.Children[0]
		nodes = append(nodes, node)
	}
	return nodes
}

// Line returns the UCI moves from the start to this node.
func (n *Node) Line() []string {
	var moves []string
	for node := n; node.Parent != nil; node = node.Parent {
		moves = append(moves, node.UCI)
	}
	for i, j := 0, len(moves)-1; i < j; i, j = i+1, j-1 {
		moves[i], moves[j] = moves[j], moves[i]
	}
	return moves
}

// Position returns the position after this node's move.
func (n *Node) Position() *chesscore.Position {
	position, err := chesscore.NewPosition(n.FEN)
	if err != nil {
		position, _ = chesscore.NewPosition("")
	}
	return position
}

// Play adds move (UCI or SAN) after n, reusing an existing child when the
// move is already there. New moves start a side line unless n had none.
func (n *Node) Play(move string) (*Node, error) {
	position := n.Position()
	uci, ok := position.ResolveMove(move)
	if !ok {
		return nil, fmt.Errorf("illegal move %q", move)
	}
	for _, child := range n.Children {
		if child.UCI == uci {
			return child, nil
		}
	}
	san := position.SAN(uci)
	if err := position.ApplyUCI(uci); err != nil {
		return nil, err
	}
	child := &Node{Parent: n, SAN: san, UCI: uci, FEN: position.FEN(), Ply: n.Ply + 1}
	n.Children = append(n.Children, child)
	return child, nil
}

// Promote makes n's line the main line at its branch point.
func (n *Node) Promote() {
	if n.Parent == nil {
		return
	}
	siblings := n.Parent.Children
	for i, sibling := range siblings {
		if sibling == n && i > 0 {
			copy(siblings[1:i+1], siblings[0:i])
			siblings[0] = n
			return
		}
	}
}

// Delete removes n and everything after it.
func (n *Node) Delete() {
	if n.Parent == nil {
		return
	}
	siblings := n.Parent.Children
	for i, sibling := range siblings {
		if sibling == n {
			n.Parent.Children = append(siblings[:i:i], siblings[i+1:]...)
			return
		}
	}
}

// MoveNumber is the full-move number shown before this move.
func (n *Node) MoveNumber(startFull int, startWhite bool) int {
	ply := n.Ply - 1
	if !startWhite {
		ply++
	}
	return startFull + ply/2
}

// WhiteMoved reports whether white played this node's move.
func (n *Node) WhiteMoved() bool {
	if n.Parent == nil {
		return false
	}
	fields := strings.Fields(n.Parent.FEN)
	return len(fields) < 2 || fields[1] == "w"
}

// FullMove returns the starting position's full-move number and side.
func (g *Game) FullMove() (int, bool) {
	fields := strings.Fields(g.Root.FEN)
	number, white := 1, true
	if len(fields) >= 6 {
		number, _ = strconv.Atoi(fields[5])
		white = fields[1] == "w"
	}
	return max(1, number), white
}

// Parse reads every game in text. Games with an illegal move keep the
// moves before it and report the problem in Err.
func Parse(text string) ([]*Game, error) {
	text = strings.ReplaceAll(strings.TrimPrefix(text, "\ufeff"), "\r\n", "\n")
	var games []*Game
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); {
		tags := map[string]string{}
		var order []string
		for ; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "" || strings.HasPrefix(line, "%") {
				continue
			}
			if !strings.HasPrefix(line, "[") {
				break
			}
			if key, value, ok := parseTag(line); ok {
				if _, seen := tags[key]; !seen {
					order = append(order, key)
				}
				tags[key] = value
			}
		}
		var body strings.Builder
		for ; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if strings.HasPrefix(trimmed, "[") && endsMovetext(body.String()) {
				break
			}
			if strings.HasPrefix(trimmed, "%") {
				continue
			}
			body.WriteString(lines[i])
			body.WriteByte('\n')
		}
		if len(tags) == 0 && strings.TrimSpace(body.String()) == "" {
			continue
		}
		games = append(games, parseGame(tags, order, body.String()))
	}
	if len(games) == 0 {
		return nil, fmt.Errorf("no PGN games found")
	}
	return games, nil
}

// endsMovetext reports whether body already holds a finished game, so a
// following tag line starts the next one.
func endsMovetext(body string) bool {
	return len(strings.Fields(body)) > 0
}

func parseTag(line string) (string, string, bool) {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
	space := strings.IndexByte(line, ' ')
	if space <= 0 {
		return "", "", false
	}
	key := line[:space]
	value := strings.TrimSpace(line[space+1:])
	if unquoted, err := strconv.Unquote(value); err == nil {
		value = unquoted
	} else {
		value = strings.Trim(value, `"`)
	}
	return key, value, true
}

func parseGame(tags map[string]string, order []string, movetext string) *Game {
	game := &Game{Tags: tags, TagOrder: order, Result: "*"}
	start, err := chesscore.NewPosition(tags["FEN"])
	if err != nil {
		start, _ = chesscore.NewPosition("")
		game.Err = fmt.Errorf("bad FEN tag: %w", err)
	}
	game.Root = &Node{FEN: start.FEN()}
	if result := tags["Result"]; result != "" {
		game.Result = result
	}

	current := game.Root
	var stack []*Node
	pending := "" // comment waiting for the next move
	tokens := tokenize(movetext)
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		switch {
		case token.comment:
			text := strings.TrimSpace(token.text)
			switch {
			case current == game.Root && len(current.Children) == 0 && len(stack) == 0:
				current.Comment = joinComment(current.Comment, text)
			case len(stack) > 0 && current == stack[len(stack)-1].Parent:
				pending = joinComment(pending, text)
			default:
				current.Comment = joinComment(current.Comment, text)
			}
		case token.text == "(":
			if current.Parent == nil {
				continue // a side line before any move: ignore the bracket
			}
			stack = append(stack, current)
			current = current.Parent
		case token.text == ")":
			if len(stack) > 0 {
				current = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			pending = ""
		case strings.HasPrefix(token.text, "$"):
			if nag, err := strconv.Atoi(token.text[1:]); err == nil && current != game.Root {
				current.NAGs = append(current.NAGs, nag)
			}
		case isResult(token.text):
			if len(stack) == 0 {
				game.Result = token.text
			}
		case isMoveNumber(token.text):
			continue
		default:
			san, nags := splitSuffix(token.text)
			if san == "" || game.Err != nil {
				continue
			}
			node, err := current.Play(san)
			if err != nil {
				game.Err = fmt.Errorf("move %s: %w", token.text, err)
				continue
			}
			node.NAGs = append(node.NAGs, nags...)
			if pending != "" {
				node.Pre, pending = pending, ""
			}
			current = node
		}
	}
	return game
}

func joinComment(a, b string) string {
	if a == "" {
		return b
	}
	return a + " " + b
}

type token struct {
	text    string
	comment bool
}

func tokenize(text string) []token {
	var tokens []token
	for i := 0; i < len(text); {
		c := text[i]
		switch {
		case c == '{':
			end := strings.IndexByte(text[i:], '}')
			if end < 0 {
				end = len(text) - i
			}
			tokens = append(tokens, token{text: strings.Join(strings.Fields(text[i+1:i+end]), " "), comment: true})
			i += end + 1
		case c == ';':
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				end = len(text) - i
			}
			tokens = append(tokens, token{text: text[i+1 : i+end], comment: true})
			i += end
		case c == '(' || c == ')':
			tokens = append(tokens, token{text: string(c)})
			i++
		case c == ' ' || c == '\n' || c == '\t' || c == '\r':
			i++
		default:
			end := i
			for end < len(text) && !strings.ContainsRune(" \n\t\r(){;", rune(text[end])) {
				end++
			}
			word := text[i:end]
			// "12.e4" and "12...Nf6" carry the move after the number.
			if dot := strings.LastIndexByte(word, '.'); dot >= 0 && dot < len(word)-1 && isMoveNumber(word[:dot+1]) {
				tokens = append(tokens, token{text: word[:dot+1]}, token{text: word[dot+1:]})
			} else {
				tokens = append(tokens, token{text: word})
			}
			i = end
		}
	}
	return tokens
}

func isMoveNumber(text string) bool {
	trimmed := strings.TrimRight(text, ".")
	if trimmed == text || trimmed == "" {
		return false
	}
	_, err := strconv.Atoi(trimmed)
	return err == nil
}

func isResult(text string) bool {
	return text == "1-0" || text == "0-1" || text == "1/2-1/2" || text == "*"
}

var suffixNAG = map[string]int{"!": 1, "?": 2, "!!": 3, "??": 4, "!?": 5, "?!": 6}

func splitSuffix(text string) (string, []int) {
	end := len(text)
	for end > 0 && (text[end-1] == '!' || text[end-1] == '?') {
		end--
	}
	suffix := text[end:]
	if nag, ok := suffixNAG[suffix]; ok {
		return text[:end], []int{nag}
	}
	return text[:end], nil
}

// Glyph renders the common move-quality NAGs as symbols.
func Glyph(nag int) string {
	for symbol, value := range suffixNAG {
		if value == nag {
			return symbol
		}
	}
	return ""
}

// String writes the game as PGN, keeping comments and side lines.
func (g *Game) String() string {
	var out strings.Builder
	keys := append([]string(nil), sevenTags...)
	rest := []string{}
	for _, key := range g.TagOrder {
		if !contains(sevenTags, key) {
			rest = append(rest, key)
		}
	}
	for key := range g.Tags {
		if !contains(sevenTags, key) && !contains(rest, key) {
			rest = append(rest, key)
		}
	}
	sort.SliceStable(rest, func(i, j int) bool { return indexOf(g.TagOrder, rest[i]) < indexOf(g.TagOrder, rest[j]) })
	for _, key := range append(keys, rest...) {
		value, ok := g.Tags[key]
		if key == "Result" {
			value, ok = g.Result, true
		}
		if !ok {
			if !contains(sevenTags, key) {
				continue
			}
			value = "?"
		}
		fmt.Fprintf(&out, "[%s %s]\n", key, strconv.Quote(value))
	}
	out.WriteByte('\n')
	var words []string
	if g.Root.Comment != "" {
		words = append(words, "{"+g.Root.Comment+"}")
	}
	full, white := g.FullMove()
	words = append(words, g.movetext(g.Root, full, white, true)...)
	words = append(words, g.Result)
	out.WriteString(wrap(words, 79))
	out.WriteByte('\n')
	return out.String()
}

func (g *Game) movetext(from *Node, full int, startWhite bool, forceNumber bool) []string {
	var words []string
	node := from
	needNumber := forceNumber
	for len(node.Children) > 0 {
		main := node.Children[0]
		words = append(words, g.moveWords(main, full, startWhite, needNumber)...)
		needNumber = main.Comment != ""
		for _, side := range node.Children[1:] {
			line := []string{"("}
			line = append(line, g.moveWords(side, full, startWhite, true)...)
			line = append(line, g.movetext(side, full, startWhite, side.Comment != "")...)
			line[len(line)-1] += ")"
			words = append(words, line[0]+line[1])
			words = append(words, line[2:]...)
			needNumber = true
		}
		node = main
	}
	return words
}

func (g *Game) moveWords(node *Node, full int, startWhite bool, forceNumber bool) []string {
	var words []string
	if node.Pre != "" {
		words = append(words, "{"+node.Pre+"}")
		forceNumber = true
	}
	number := node.MoveNumber(full, startWhite)
	white := node.WhiteMoved()
	switch {
	case white:
		words = append(words, fmt.Sprintf("%d.", number))
	case forceNumber:
		words = append(words, fmt.Sprintf("%d...", number))
	}
	move := node.SAN
	var extra []string
	for _, nag := range node.NAGs {
		if glyph := Glyph(nag); glyph != "" && !strings.ContainsAny(move, "!?") {
			move += glyph
		} else {
			extra = append(extra, fmt.Sprintf("$%d", nag))
		}
	}
	words = append(words, move)
	words = append(words, extra...)
	if node.Comment != "" {
		words = append(words, "{"+node.Comment+"}")
	}
	return words
}

func wrap(words []string, width int) string {
	var out, line strings.Builder
	for _, word := range words {
		if line.Len() > 0 && line.Len()+1+len(word) > width {
			out.WriteString(line.String())
			out.WriteByte('\n')
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(word)
	}
	out.WriteString(line.String())
	return out.String()
}

func contains(list []string, value string) bool { return indexOf(list, value) >= 0 }

func indexOf(list []string, value string) int {
	for i, item := range list {
		if item == value {
			return i
		}
	}
	return len(list) + 1000
}

// Title is a short "White – Black" or Event name for lists.
func (g *Game) Title() string {
	white, black := g.Tags["White"], g.Tags["Black"]
	if white != "" && white != "?" && black != "" && black != "?" {
		return white + " – " + black
	}
	if event := g.Tags["Event"]; event != "" && event != "?" {
		return event
	}
	if chapter := g.Tags["ChapterName"]; chapter != "" {
		return chapter
	}
	return "Untitled game"
}
