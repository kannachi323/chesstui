package chess

import (
	"fmt"
	"strings"

	chessongo "github.com/chesshere-com/chess-on-go"
)

const standardFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type Color uint8

const (
	NoColor Color = iota
	White
	Black
)

type Piece struct {
	Rune  rune
	Color Color
}

// Position owns a validated game state and its UCI move history.
type Position struct {
	game       *chessongo.Game
	initialFEN string
	moves      []string
}

func NewPosition(fen string) (*Position, error) {
	if fen == "" || fen == "startpos" {
		fen = standardFEN
	}
	game, err := chessongo.NewGameFromFEN(fen)
	if err != nil {
		return nil, fmt.Errorf("parse FEN: %w", err)
	}
	return &Position{game: game, initialFEN: fen}, nil
}

func NewChess960Position(fen string) (*Position, error) {
	game, err := chessongo.NewGameFromFENWithVariant(fen, chessongo.VariantChess960)
	if err != nil {
		return nil, fmt.Errorf("parse Chess960 FEN: %w", err)
	}
	return &Position{game: game, initialFEN: fen}, nil
}

func (p *Position) Clone() *Position {
	moves := append([]string(nil), p.moves...)
	return &Position{game: p.game.Clone(), initialFEN: p.initialFEN, moves: moves}
}

func (p *Position) ApplyUCI(move string) error {
	move = strings.TrimSpace(strings.ToLower(move))
	if err := p.game.TryMoveUCI(move); err != nil {
		return fmt.Errorf("illegal UCI move %q: %w", move, err)
	}
	p.moves = append(p.moves, move)
	return nil
}

func (p *Position) ApplySAN(move string) error {
	parsed, err := p.game.ParseMoveSAN(strings.TrimSpace(move))
	if err != nil {
		return fmt.Errorf("invalid SAN move %q: %w", move, err)
	}
	if err := p.game.TryMove(parsed); err != nil {
		return fmt.Errorf("illegal SAN move %q: %w", move, err)
	}
	p.moves = append(p.moves, parsed.UCI())
	return nil
}

func (p *Position) ApplyMoves(moves string) error {
	for _, move := range strings.Fields(moves) {
		if err := p.ApplyUCI(move); err != nil {
			return err
		}
	}
	return nil
}

func (p *Position) AtPly(ply int) (*Position, error) {
	if ply < 0 || ply > len(p.moves) {
		return nil, fmt.Errorf("ply %d outside move history", ply)
	}
	position, err := NewPosition(p.initialFEN)
	if err != nil {
		return nil, err
	}
	for _, move := range p.moves[:ply] {
		if err := position.ApplyUCI(move); err != nil {
			return nil, err
		}
	}
	return position, nil
}

func (p *Position) PieceAt(file, rank int) Piece {
	if file < 0 || file > 7 || rank < 0 || rank > 7 {
		return Piece{}
	}
	board := p.game.Board()
	// chess-on-go stores its board from a8 to h1; the adapter exposes normal
	// zero-based chess coordinates where rank 0 is rank one.
	value := board[(7-rank)*8+file]
	if value.Kind() == chessongo.EMPTY {
		return Piece{}
	}
	color := White
	if value.IsBlack() {
		color = Black
	}
	return Piece{Rune: value.ToRune(), Color: color}
}

func (p *Position) LegalMoves() []string {
	moves := p.game.LegalMovesInto(nil)
	result := make([]string, 0, len(moves))
	for _, move := range moves {
		result = append(result, move.UCI())
	}
	return result
}

func (p *Position) FEN() string        { return p.game.FEN() }
func (p *Position) InitialFEN() string { return p.initialFEN }
func (p *Position) Moves() []string    { return append([]string(nil), p.moves...) }
func (p *Position) Ply() int           { return len(p.moves) }
func (p *Position) InCheck() bool      { return p.game.ComputeIsCheck() }
func (p *Position) Terminal() bool     { return p.game.IsTerminal() }

func (p *Position) Turn() Color {
	if p.game.SideToMove() == chessongo.WHITE {
		return White
	}
	return Black
}
