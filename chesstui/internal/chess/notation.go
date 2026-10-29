package chess

import (
	"fmt"
	"strings"
	"unicode"

	chessongo "github.com/chesshere-com/chess-on-go"
)

// SAN renders a legal UCI move from this position in standard algebraic
// notation, for example e2e4 → e4, g1f3 → Nf3, e1g1 → O-O.
func (p *Position) SAN(uci string) string {
	uci = strings.ToLower(strings.TrimSpace(uci))
	if len(uci) < 4 {
		return uci
	}
	ff, fr, okFrom := parseSquare(uci[:2])
	tf, tr, okTo := parseSquare(uci[2:4])
	if !okFrom || !okTo {
		return uci
	}
	piece := p.PieceAt(ff, fr)
	if piece.Rune == 0 {
		return uci
	}
	kind := unicode.ToUpper(piece.Rune)
	target := p.PieceAt(tf, tr)
	var san strings.Builder
	switch {
	case kind == 'K' && (tf-ff == 2 || ff-tf == 2 || (target.Rune != 0 && target.Color == piece.Color)):
		if tf > ff {
			san.WriteString("O-O")
		} else {
			san.WriteString("O-O-O")
		}
	case kind == 'P':
		if ff != tf {
			san.WriteByte(uci[0])
			san.WriteByte('x')
		}
		san.WriteString(uci[2:4])
		if len(uci) == 5 {
			san.WriteByte('=')
			san.WriteRune(unicode.ToUpper(rune(uci[4])))
		}
	default:
		san.WriteRune(kind)
		san.WriteString(p.disambiguation(uci, kind, piece.Color))
		if target.Rune != 0 {
			san.WriteByte('x')
		}
		san.WriteString(uci[2:4])
	}
	after := p.Clone()
	if err := after.ApplyUCI(uci); err != nil {
		return uci
	}
	if after.game.Status() == chessongo.GameStatusCheckmate {
		san.WriteByte('#')
	} else if after.InCheck() {
		san.WriteByte('+')
	}
	return san.String()
}

func (p *Position) disambiguation(uci string, kind rune, color Color) string {
	sameFile, sameRank, rivals := false, false, false
	for _, other := range p.LegalMoves() {
		if other[2:4] != uci[2:4] || other[:2] == uci[:2] {
			continue
		}
		of, or, _ := parseSquare(other[:2])
		rival := p.PieceAt(of, or)
		if unicode.ToUpper(rival.Rune) != kind || rival.Color != color {
			continue
		}
		rivals = true
		sameFile = sameFile || other[0] == uci[0]
		sameRank = sameRank || other[1] == uci[1]
	}
	switch {
	case !rivals:
		return ""
	case !sameFile:
		return uci[:1]
	case !sameRank:
		return uci[1:2]
	default:
		return uci[:2]
	}
}

// SANMoves returns the move history in algebraic notation.
func (p *Position) SANMoves() []string {
	replay, err := NewPosition(p.initialFEN)
	if err != nil {
		return p.Moves()
	}
	result := make([]string, 0, len(p.moves))
	for _, move := range p.moves {
		result = append(result, replay.SAN(move))
		if replay.ApplyUCI(move) != nil {
			break
		}
	}
	return result
}

// KingSquare returns the square of the given side's king, or "".
func (p *Position) KingSquare(color Color) string {
	want := 'K'
	if color == Black {
		want = 'k'
	}
	for rank := range 8 {
		for file := range 8 {
			if p.PieceAt(file, rank).Rune == want {
				return SquareName(file, rank)
			}
		}
	}
	return ""
}

// Outcome describes a finished game, or returns "" while it is still going.
func (p *Position) Outcome() string {
	switch p.game.Status() {
	case chessongo.GameStatusCheckmate:
		if p.game.Winner() == chessongo.WHITE {
			return "Checkmate · white wins"
		}
		return "Checkmate · black wins"
	case chessongo.GameStatusStalemate:
		return "Stalemate · draw"
	case chessongo.GameStatusDrawInsufficientMaterial:
		return "Draw · insufficient material"
	case chessongo.GameStatusDrawFivefoldRepetition:
		return "Draw · repetition"
	case chessongo.GameStatusDrawSeventyFiveMoveRule:
		return "Draw · 75-move rule"
	}
	if p.Terminal() {
		return "Game over"
	}
	return ""
}

// SquareName converts zero-based file and rank to a name such as "e4".
func SquareName(file, rank int) string {
	return string([]byte{byte('a' + file), byte('1' + rank)})
}

func parseSquare(name string) (file, rank int, ok bool) {
	if len(name) != 2 || name[0] < 'a' || name[0] > 'h' || name[1] < '1' || name[1] > '8' {
		return 0, 0, false
	}
	return int(name[0] - 'a'), int(name[1] - '1'), true
}

// ParseSquare is the exported form of parseSquare for UI code.
func ParseSquare(name string) (file, rank int, ok bool) { return parseSquare(name) }

// ResolveMove accepts UCI (e2e4, e7e8q) or SAN (e4, Nf3, exd5, O-O) and
// returns the legal move in UCI. A four-character pawn move to the last rank
// promotes to a queen.
func (p *Position) ResolveMove(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", false
	}
	lower := strings.ToLower(input)
	legal := p.LegalMoves()
	for _, candidate := range []string{lower, lower + "q"} {
		for _, move := range legal {
			if move == candidate {
				return move, true
			}
		}
	}
	san := strings.ReplaceAll(input, "0", "O")
	if parsed, err := p.game.ParseMoveSAN(san); err == nil {
		return parsed.UCI(), true
	}
	// Players often type pawn captures and piece moves in lower case.
	if len(input) > 1 && strings.ContainsRune("nbrqk", rune(input[0])) {
		if parsed, err := p.game.ParseMoveSAN(strings.ToUpper(input[:1]) + input[1:]); err == nil {
			return parsed.UCI(), true
		}
	}
	return "", false
}

// NewPositionTurn returns the side to move in fen, or White if it is invalid.
func NewPositionTurn(fen string) Color {
	if fields := strings.Fields(fen); len(fields) > 1 && fields[1] == "b" {
		return Black
	}
	return White
}

// ApplyNotation plays a space-separated move list in UCI or SAN, as found in
// Lichess exports (SAN) and Board API streams (UCI).
func (p *Position) ApplyNotation(moves string) error {
	for _, move := range strings.Fields(moves) {
		uci, ok := p.ResolveMove(move)
		if !ok {
			return fmt.Errorf("illegal move %q", move)
		}
		if err := p.ApplyUCI(uci); err != nil {
			return err
		}
	}
	return nil
}
