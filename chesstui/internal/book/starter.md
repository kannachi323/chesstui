# First Moves
by chesstui

## How to read this book

This is a small book about the ideas that decide most games between club players: develop quickly, keep your king safe, and use open lines. It is written for reading in a terminal split while something else compiles.

The board beside the text follows you. Press → to play the next move in the text and ← to take it back. Scrolling with j and k moves the board to whatever you are reading. Press a at any point to open the game in Analyze and try your own ideas.

- t opens the table of contents and your bookmarks.
- m bookmarks the current move.
- b hides or shows the board, and z hides everything but the page.

Your place is saved as you read, so you can close the terminal and pick up where you stopped.

## Develop, castle, connect

In 1858, at the Paris opera, Paul Morphy played a casual game against two amateurs, the Duke of Brunswick and Count Isouard. It has become the standard example of what quick development does to an opponent who spends moves on other things.

Watch how every white move brings a new piece into play, while Black twice moves pieces that are already out.

```pgn
[Event "Paris"]
[Date "1858.??.??"]
[White "Paul Morphy"]
[Black "Duke Karl / Count Isouard"]
[Result "1-0"]

1. e4 e5 2. Nf3 d6 {The Philidor Defence: solid, but a little passive.}
3. d4 Bg4?! {Pinning the knight looks active, but Black will soon have to
give up this bishop.} (3... exd4 {keeps the position closed and is the main
line today.}) 4. dxe5 Bxf3 5. Qxf3 dxe5 6. Bc4 {Two developed pieces aim at
f7, the weakest square in Black's camp.} 6... Nf6? 7. Qb3 {A double attack on
f7 and b7.} 7... Qe7 8. Nc3 {Morphy declines the pawn on b7 and develops
instead.} 8... c6 9. Bg5 b5? 10. Nxb5! {White gives a piece to rip the
queenside open while Black's pieces are still at home.} 10... cxb5 11. Bxb5+
Nbd7 12. O-O-O {Castling brings the rook straight to the open d-file.}
12... Rd8 13. Rxd7 Rxd7 14. Rd1 Qe6 15. Bxd7+ Nxd7 16. Qb8+!! {The queen
sacrifice that finishes the game.} 16... Nxb8 17. Rd8# 1-0
```

Count the developed pieces after move 12: White has everything in play and the king is safe. Black's king is still in the centre, the h8-rook has never moved, and the bishop on f8 has not seen the game.

## Traps and the f7 square

Before castling, the squares next to the king are protected by the king alone. The f7 square for Black, and f2 for White, are the classic targets. This trap is named after Légal, Philidor's teacher in the Café de la Régence.

```pgn
[Event "Légal's mate"]
[Result "1-0"]

1. e4 e5 2. Nf3 d6 3. Bc4 Bg4 4. Nc3 g6? {Too slow. Black spends a move on a
pawn while White's pieces are ready.} 5. Nxe5! {It looks like White simply
loses the queen.} 5... Bxd1? {Taking it loses at once.} (5... dxe5 6. Qxg4
{and White is a pawn up with a strong position.}) 6. Bxf7+ Ke7 7. Nd5# 1-0
```

Three minor pieces deliver mate while the white queen sits captured on d1. The lesson is not the trap itself, but the question to ask before every capture: what does my opponent do next?

## The opposition

With only kings and one pawn left, the side to move often decides the result. When the two kings stand face to face with one square between them, the player who does not have to move is said to have the opposition.

```fen
8/8/8/4k3/8/4K3/4P3/8 w - - 0 1
White to move. The kings face each other, and White must give way.
```

Here White would like to advance, but every king move lets the black king step forward. The pawn helps: it can spend a move instead of the king.

```pgn
[Event "Opposition"]
[SetUp "1"]
[FEN "8/8/8/4k3/8/4K3/4P3/8 w - - 0 1"]
[Result "*"]

1. Kf3 {Sidestepping.} 1... Kf5 {Black keeps the opposition.} 2. e4+ {The pawn
move hands the duty to move back to Black.} 2... Ke5 3. Ke3 {Now White has the
opposition: the kings face each other and Black must move.} 3... Ke6 4. Kd4
Kd6 5. e5+ Ke6 6. Ke4 *
```

The rule of thumb: get your king in front of your pawn, and use spare pawn moves to win the opposition when you need it.

```fen
8/8/4k3/8/4PK2/8/8/8 b - - 0 1
Black to move. Can the black king stop the pawn?
```

## Write your own book

A book is a plain Markdown file. Put it in the books folder (Settings shows where the library lives) or open it from the Read library with o.

- A line starting with one hash sign names the book, and "by" on the next line names the author.
- Two hash signs start a chapter, and three start a section heading.
- A fenced block marked pgn holds a game. Comments in braces and side lines in brackets become part of the text.
- A fenced block marked fen holds a diagram: the FEN on the first line and a caption after it.

Annotated PGN files work too: every game becomes a chapter, and its comments are the prose. A Lichess study downloaded as PGN reads this way, and the Games section of the Lichess tab saves a study straight into your library.
