import { Chess } from 'chess.js'
import { describe, expect, it } from 'vitest'
import { legalTargets, moveBoardCursor, resolveBoardDrop, resolveMoveInput } from './moves'

describe('resolveMoveInput', () => {
  const afterE4 = 'rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1'

  it('accepts UCI', () => {
    expect(resolveMoveInput(afterE4, 'e7e5')).toBe('e7e5')
  })

  it('accepts SAN', () => {
    expect(resolveMoveInput(afterE4, 'e5')).toBe('e7e5')
  })

  it('accepts a resulting FEN', () => {
    const target = new Chess(afterE4)
    target.move('e5')
    expect(resolveMoveInput(afterE4, target.fen())).toBe('e7e5')
  })

  it('rejects a FEN that is not one move away', () => {
    expect(() => resolveMoveInput(afterE4, new Chess().fen())).toThrow(/not reachable/i)
  })

  it('finds legal cursor destinations and resolves a drop', () => {
    expect(legalTargets(afterE4, 'e7')).toEqual(expect.arrayContaining(['e6', 'e5']))
    expect(resolveBoardDrop(afterE4, 'e7', 'e5')).toBe('e7e5')
  })

  it('moves the cursor in screen directions for either orientation', () => {
    expect(moveBoardCursor('e4', 'h', 'white')).toBe('d4')
    expect(moveBoardCursor('e4', 'k', 'white')).toBe('e5')
    expect(moveBoardCursor('e4', 'h', 'black')).toBe('f4')
    expect(moveBoardCursor('e4', 'k', 'black')).toBe('e3')
  })
})
