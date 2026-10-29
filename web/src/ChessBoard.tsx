import { createElement, useEffect, useRef, type CSSProperties } from 'react'
import type { ChessBoardElement } from 'chessboard-element'
import 'chessboard-element'

const files = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h']
const ranks = ['8', '7', '6', '5', '4', '3', '2', '1']

export function ChessBoard({
  fen,
  orientation = 'white',
  label = 'Chess position',
  cursor,
  selected,
  legalMoves = [],
}: {
  fen: string
  orientation?: 'white' | 'black'
  label?: string
  cursor?: string
  selected?: string
  legalMoves?: string[]
}) {
  const boardRef = useRef<ChessBoardElement | null>(null)

  useEffect(() => {
    if (!boardRef.current) return
    boardRef.current.orientation = orientation
    boardRef.current.hideNotation = true
    boardRef.current.setPosition(fen.split(' ')[0], false)
  }, [fen, orientation])

  const board = createElement('chess-board', {
    ref: (node: Element | null) => { boardRef.current = node as ChessBoardElement | null },
    className: 'piece-board',
    'aria-label': label,
    style: {
      '--light-color': '#A9B88E',
      '--dark-color': '#596A50',
      '--highlight-color': '#8AADF4',
    } as CSSProperties,
  })

  const displayFiles = orientation === 'white' ? files : [...files].reverse()
  const displayRanks = orientation === 'white' ? ranks : [...ranks].reverse()
  const displaySquares = displayRanks.flatMap((rank) => displayFiles.map((file) => `${file}${rank}`))

  return (
    <div className="board-frame">
      <div className="rank-labels" aria-hidden="true">
        {displayRanks.map((rank) => <span key={rank}>{rank}</span>)}
      </div>
      <div className="board-stage">
        {board}
        {cursor ? <div className="board-cursor-layer" aria-hidden="true">
          {displaySquares.map((square) => <span
            className={`${cursor === square ? 'cursor-square' : ''} ${selected === square ? 'selected-square' : ''} ${legalMoves.includes(square) ? 'legal-square' : ''}`}
            key={square}
          />)}
        </div> : null}
      </div>
      <div className="file-labels" aria-hidden="true">
        {displayFiles.map((file) => <span key={file}>{file}</span>)}
      </div>
    </div>
  )
}
