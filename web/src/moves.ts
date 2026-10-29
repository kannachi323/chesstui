import { Chess, type Move, type Square } from 'chess.js'

export function resolveMoveInput(fen: string, input: string) {
  const raw = input.trim()
  if (!raw) throw new Error('Enter a move.')
  const position = new Chess(fen === 'start' ? undefined : fen)

  if (/^[a-h][1-8][a-h][1-8][qrbn]?$/i.test(raw)) {
    const uci = raw.toLowerCase()
    const move = position.move({
      from: uci.slice(0, 2),
      to: uci.slice(2, 4),
      promotion: uci.slice(4, 5) || 'q',
    })
    return toUCI(move)
  }

  if (looksLikeFen(raw)) {
    const target = new Chess(raw)
    const targetPosition = comparableFen(target.fen())
    for (const candidate of position.moves({ verbose: true })) {
      const next = new Chess(position.fen())
      next.move(candidate)
      if (comparableFen(next.fen()) === targetPosition) return toUCI(candidate)
    }
    throw new Error('That FEN is not reachable with one legal move.')
  }

  return toUCI(position.move(raw))
}

export function legalTargets(fen: string, source: string) {
  const position = new Chess(fen === 'start' ? undefined : fen)
  return [...new Set(position.moves({ square: source as Square, verbose: true }).map((move) => move.to))]
}

export function resolveBoardDrop(fen: string, source: string, target: string) {
  const position = new Chess(fen === 'start' ? undefined : fen)
  const candidates = position.moves({ square: source as Square, verbose: true }).filter((move) => move.to === target)
  const move = candidates.find((candidate) => candidate.promotion === 'q') || candidates.find((candidate) => !candidate.promotion)
  if (!move) throw new Error(`${source} cannot move to ${target}.`)
  return toUCI(move)
}

export function moveBoardCursor(square: string, key: string, orientation: 'white' | 'black') {
  const file = square.charCodeAt(0) - 97
  const rank = Number(square[1]) - 1
  const direction = orientation === 'white' ? 1 : -1
  let nextFile = file
  let nextRank = rank
  if (key === 'h' || key === 'ArrowLeft') nextFile -= direction
  if (key === 'l' || key === 'ArrowRight') nextFile += direction
  if (key === 'k' || key === 'ArrowUp') nextRank += direction
  if (key === 'j' || key === 'ArrowDown') nextRank -= direction
  nextFile = Math.max(0, Math.min(7, nextFile))
  nextRank = Math.max(0, Math.min(7, nextRank))
  return `${String.fromCharCode(97 + nextFile)}${nextRank + 1}`
}

function toUCI(move: Move) {
  return `${move.from}${move.to}${move.promotion || ''}`
}

function looksLikeFen(value: string) {
  return value.trim().split(/\s+/).length === 6 && value.includes('/')
}

function comparableFen(value: string) {
  return value.split(/\s+/).slice(0, 4).join(' ')
}
