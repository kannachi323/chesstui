import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  account: vi.fn(),
  beginLogin: vi.fn(),
  clearToken: vi.fn(),
  createAIGame: vi.fn(),
  createUserChallenge: vi.fn(),
  currentGames: vi.fn(),
  dailyPuzzle: vi.fn(),
  endBoardGame: vi.fn(),
  finishLogin: vi.fn(),
  makeBoardMove: vi.fn(),
  nextPuzzle: vi.fn(),
  streamBoardGame: vi.fn(),
  streamTV: vi.fn(),
  streamUserEvents: vi.fn(),
  watchDirectory: vi.fn(),
}))

vi.mock('./lichess', () => api)
vi.mock('./ChessBoard', () => ({ ChessBoard: ({ label }: { label?: string }) => <div aria-label={label || 'Chess position'} /> }))

import App from './App'

const account = { id: 'kannachi', username: 'kannachi' }
const gameFull = {
  type: 'gameFull',
  id: 'testgame',
  initialFen: 'startpos',
  white: { id: 'kannachi', name: 'kannachi', rating: 1500 },
  black: { id: 'lichess', name: 'Lichess AI level 3' },
  state: { type: 'gameState', moves: '', wtime: 300000, btime: 300000, winc: 3000, binc: 3000, status: 'started' },
}

describe('ChessTUI browser shell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.finishLogin.mockResolvedValue('token')
    api.account.mockResolvedValue(account)
    api.currentGames.mockResolvedValue({ nowPlaying: [] })
    api.createAIGame.mockResolvedValue({ id: 'testgame', player: 'white', fen: 'start' })
    api.makeBoardMove.mockResolvedValue({ ok: true })
    api.endBoardGame.mockResolvedValue({ ok: true })
    api.streamBoardGame.mockImplementation((_token, _id, _signal, update) => {
      update(gameFull)
      return Promise.resolve()
    })
  })

  it('hides sign in after authentication and keeps jk navigation reversible', async () => {
    render(<App />)
    await screen.findByText(/connected to Lichess/i)

    fireEvent.keyDown(window, { key: '2' })
    expect(await screen.findByText('Play Lichess AI level 3')).toBeTruthy()
    expect(screen.queryByText('Connect Lichess account')).toBeNull()
    expect(screen.getByText('Disconnect kannachi')).toBeTruthy()

    fireEvent.keyDown(window, { key: '1' })
    fireEvent.keyDown(window, { key: 'Tab' })
    fireEvent.keyDown(window, { key: 'j' })
    fireEvent.keyDown(window, { key: 'k' })
    fireEvent.keyDown(window, { key: 'l' })
    fireEvent.keyDown(window, { key: 'Enter' })
    expect(await screen.findByRole('heading', { name: 'Play' })).toBeTruthy()
  })

  it('creates an AI game and submits a legal move through the Board API', async () => {
    render(<App />)
    await screen.findByText(/connected to Lichess/i)
    fireEvent.keyDown(window, { key: '2' })
    fireEvent.keyDown(window, { key: 'b' })

    expect(await screen.findByText('kannachi vs Lichess AI level 3')).toBeTruthy()
    expect(api.createAIGame).toHaveBeenCalledWith('token', 3)
    expect(api.streamBoardGame).toHaveBeenCalled()

    fireEvent.keyDown(window, { key: 'i' })
    for (const key of 'e2e4') fireEvent.keyDown(window, { key })
    fireEvent.keyDown(window, { key: 'Enter' })

    await waitFor(() => expect(api.makeBoardMove).toHaveBeenCalledWith('token', 'testgame', 'e2e4'))
    expect(screen.getByText('NAV')).toBeTruthy()
  })

  it('selects and drops a piece using the NAV board cursor', async () => {
    render(<App />)
    await screen.findByText(/connected to Lichess/i)
    fireEvent.keyDown(window, { key: '2' })
    fireEvent.keyDown(window, { key: 'b' })
    await screen.findByText('kannachi vs Lichess AI level 3')

    fireEvent.keyDown(window, { key: 'Enter' })
    expect(await screen.findByText(/e2 selected/i)).toBeTruthy()
    fireEvent.keyDown(window, { key: 'k' })
    fireEvent.keyDown(window, { key: 'k' })
    fireEvent.keyDown(window, { key: 'Enter' })

    await waitFor(() => expect(api.makeBoardMove).toHaveBeenCalledWith('token', 'testgame', 'e2e4'))
  })
})
