import { useCallback, useEffect, useRef, useState } from 'react'
import { Chess } from 'chess.js'
import { ChessBoard } from './ChessBoard'
import { legalTargets, moveBoardCursor, resolveBoardDrop, resolveMoveInput } from './moves'
import {
  account as fetchAccount,
  beginLogin,
  clearToken,
  createAIGame,
  createUserChallenge,
  currentGames,
  dailyPuzzle,
  endBoardGame,
  finishLogin,
  makeBoardMove,
  nextPuzzle,
  streamBoardGame,
  streamTV,
  streamUserEvents,
  watchDirectory,
  type Account,
  type BoardGameEvent,
  type Broadcast,
  type CreatedGame,
  type GameFullEvent,
  type GameStateEvent,
  type PuzzlePayload,
  type Streamer,
  type TVChannel,
  type TVEvent,
} from './lichess'

const workspaces = ['Home', 'Play', 'Watch', 'Puzzles', 'Study', 'Learn', 'Settings']
const statuses = [
  'local-first • data stays on this machine',
  'a connect • b Lichess AI • u challenge • x abort/resign',
  '[ ] source • j/k select • enter watch/open • r refresh',
  'd daily • n Lichess • h hint • i insert',
  'j/k select • enter open',
  'j/k lesson • i insert • enter submit',
  'web settings • GitHub links',
]
const wordmark = ` █▀▀ █ █ █▀▀ █▀▀ █▀▀ ▀█▀ █ █ █
 █   █▀█ ██▄ ▄▄█ ▄▄█  █  █ █ █
 ▀▀▀ ▀ ▀ ▀▀▀ ▀▀▀ ▀▀▀  ▀  ▀▀▀ ▀`

type InputMode = 'nav' | 'insert'
type KeyHandler = (event: KeyboardEvent, mode: InputMode) => boolean
type RegisterKeys = (handler: KeyHandler) => () => void

export default function App() {
  const [active, setActive] = useState(0)
  const [nav, setNav] = useState(0)
  const [focus, setFocus] = useState<'content' | 'navigation'>('content')
  const [mode, setMode] = useState<InputMode>('nav')
  const [commandOpen, setCommandOpen] = useState(false)
  const [command, setCommand] = useState('')
  const [pending, setPending] = useState<'g' | 'ctrl+w' | null>(null)
  const [help, setHelp] = useState(false)
  const [token, setToken] = useState('')
  const [account, setAccount] = useState<Account | null>(null)
  const [authError, setAuthError] = useState('')
  const [quit, setQuit] = useState(false)
  const screenKeys = useRef<KeyHandler | null>(null)
  const terminal = useRef<HTMLDivElement>(null)

  const registerKeys = useCallback<RegisterKeys>((handler) => {
    screenKeys.current = handler
    return () => {
      if (screenKeys.current === handler) screenKeys.current = null
    }
  }, [])

  const activate = useCallback((index: number) => {
    setActive(index)
    setNav(index)
    setHelp(false)
  }, [])

  const switchWorkspace = useCallback((delta: number) => {
    setActive((current) => {
      const next = (current + delta + workspaces.length) % workspaces.length
      setNav(next)
      return next
    })
    setHelp(false)
  }, [])

  useEffect(() => {
    finishLogin()
      .then(async (nextToken) => {
        if (!nextToken) return
        setToken(nextToken)
        setAccount(await fetchAccount(nextToken))
        setAuthError('')
      })
      .catch((error: unknown) => setAuthError(error instanceof Error ? error.message : String(error)))
  }, [])

  useEffect(() => {
    if (!pending) return
    const timeout = window.setTimeout(() => setPending(null), 650)
    return () => window.clearTimeout(timeout)
  }, [pending])

  useEffect(() => {
    const executeCommand = (value: string) => {
      const normalized = value.trim().replace(/^:/, '').toLowerCase()
      if (normalized === 'help' || normalized === 'h') setHelp(true)
      else if (normalized === 'tn' || normalized === 'tabnext') switchWorkspace(1)
      else if (['tp', 'tabprevious', 'tabprev'].includes(normalized)) switchWorkspace(-1)
      else if (['nav', 'navigation'].includes(normalized)) { setFocus('navigation'); setNav(active) }
      else if (normalized === 'content') setFocus('content')
      else if (['q', 'quit', 'qa', 'qall'].includes(normalized)) setQuit(true)
      else {
        const index = workspaces.findIndex((label) => label.toLowerCase() === normalized)
        if (index >= 0) activate(index)
      }
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (quit) {
        event.preventDefault()
        return
      }

      if (commandOpen) {
        if (event.key === 'Escape') { setCommandOpen(false); setCommand('') }
        else if (event.key === 'Backspace') setCommand((value) => value.slice(0, -1))
        else if (event.key === 'Enter') { executeCommand(command); setCommandOpen(false); setCommand('') }
        else if (event.key.length === 1) setCommand((value) => value + event.key)
        event.preventDefault()
        return
      }

      if (mode === 'insert') {
        if (event.key === 'Escape') {
          setMode('nav')
          setPending(null)
          event.preventDefault()
          return
        }
        const handled = focus === 'content' && !help && Boolean(screenKeys.current?.(event, 'insert'))
        if (handled || event.key.length === 1 || ['Backspace', 'Enter', 'Tab'].includes(event.key)) event.preventDefault()
        return
      }

      if (pending) {
        let handled = false
        if (pending === 'g') {
          if (event.key === 't') { switchWorkspace(1); handled = true }
          else if (event.key === 'T') { switchWorkspace(-1); handled = true }
          else if (event.key === 'g') { setFocus('navigation'); setNav(0); handled = true }
        } else if (['k', 'h'].includes(event.key.toLowerCase())) {
          setFocus('navigation'); setNav(active); handled = true
        } else if (['j', 'l'].includes(event.key.toLowerCase())) { setFocus('content'); handled = true }
        setPending(null)
        if (handled) {
          event.preventDefault()
          return
        }
      }

      if (event.key === ':') { setCommandOpen(true); setCommand(''); event.preventDefault(); return }
      if (event.key === 'i') { setMode('insert'); setFocus('content'); setPending(null); event.preventDefault(); return }
      if (event.key === '?') { setHelp((value) => !value); event.preventDefault(); return }
      if (event.key === 'Tab') {
        setFocus((current) => {
          const next = current === 'content' ? 'navigation' : 'content'
          if (next === 'navigation') setNav(active)
          return next
        })
        event.preventDefault()
        return
      }
      if (event.key === 'Escape') {
        if (focus === 'content' && !help && screenKeys.current?.(event, 'nav')) {
          event.preventDefault()
          return
        }
        setHelp(false); setFocus('navigation'); setNav(active); event.preventDefault(); return
      }
      if (event.key === 'H') { switchWorkspace(-1); event.preventDefault(); return }
      if (event.key === 'L') { switchWorkspace(1); event.preventDefault(); return }

      if (focus === 'content' && !help && screenKeys.current?.(event, 'nav')) {
        event.preventDefault()
        return
      }
      if (/^[1-7]$/.test(event.key)) { activate(Number(event.key) - 1); event.preventDefault(); return }
      if (event.ctrlKey && event.key.toLowerCase() === 'w') {
        setPending('ctrl+w'); event.preventDefault(); return
      }
      if (event.key === 'g') { setPending('g'); event.preventDefault(); return }

      if (focus === 'navigation') {
        if (['h', 'k', 'ArrowLeft', 'ArrowUp'].includes(event.key)) setNav((value) => (value + 6) % 7)
        else if (['j', 'l', 'ArrowRight', 'ArrowDown'].includes(event.key)) setNav((value) => (value + 1) % 7)
        else if (event.key === '0' || event.key === 'Home') setNav(0)
        else if (event.key === '$' || event.key === 'End') setNav(6)
        else if (['Enter', ' '].includes(event.key)) { activate(nav); setFocus('content') }
        else return
        event.preventDefault()
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [active, activate, command, commandOpen, focus, help, mode, nav, pending, quit, switchWorkspace])

  const disconnect = () => {
    clearToken()
    setToken('')
    setAccount(null)
  }

  return (
    <main className="app-shell">
      <div
        className="terminal"
        ref={terminal}
        tabIndex={0}
        onClick={() => terminal.current?.focus()}
        aria-label="ChessTUI web application"
      >
        <header className="tui-header">
          <a className="brand" href="https://github.com/kannachi323/chess-cli" target="_blank" rel="noreferrer">♞&nbsp; ChessTUI</a>
          <nav aria-label="Workspaces">
            {workspaces.map((label, index) => (
              <button
                key={label}
                type="button"
                className={`${active === index ? 'active' : ''} ${focus === 'navigation' && nav === index ? 'cursor' : ''}`}
                onClick={() => { activate(index); setFocus('content'); setMode('nav') }}
              >{label}</button>
            ))}
          </nav>
        </header>

        <section className="viewport">
          {quit ? <div className="plain-screen"><h1>ChessTUI session closed.</h1><p className="muted">Reload the page to start another session.</p></div> : help ? <HelpScreen /> : (
            <Screen
              active={active}
              activate={activate}
              registerKeys={registerKeys}
              token={token}
              account={account}
              authError={authError}
              disconnect={disconnect}
              setMode={setMode}
            />
          )}
        </section>

        <footer className="statusline">
          <span className={`mode ${mode}`}>{mode.toUpperCase()}</span>
          <span className="key-map">{commandOpen ? `:${command}_` : mode === 'insert' ? 'type freely  •  esc nav' : pending ? `${pending === 'g' ? 'g' : '^W'}_` : 'hjkl move  •  i insert  •  gt/gT tabs  •  : command'}</span>
          <span className="context-status">{help ? '? close help' : active === 1 && account ? 'b Lichess AI • u challenge • x abort/resign' : statuses[active]}</span>
        </footer>
      </div>
    </main>
  )
}

function Screen(props: {
  active: number
  activate: (index: number) => void
  registerKeys: RegisterKeys
  token: string
  account: Account | null
  authError: string
  disconnect: () => void
  setMode: (mode: InputMode) => void
}) {
  switch (props.active) {
    case 0: return <HomeScreen activate={props.activate} account={props.account} />
    case 1: return <PlayScreen registerKeys={props.registerKeys} token={props.token} account={props.account} authError={props.authError} disconnect={props.disconnect} setMode={props.setMode} />
    case 2: return <WatchScreen registerKeys={props.registerKeys} />
    case 3: return <PuzzleScreen registerKeys={props.registerKeys} token={props.token} />
    case 4: return <StudyScreen registerKeys={props.registerKeys} account={props.account} />
    case 5: return <LearnScreen registerKeys={props.registerKeys} />
    default: return <SettingsScreen />
  }
}

function HomeScreen({ activate, account }: { activate: (index: number) => void; account: Account | null }) {
  const actions = ['Play a game', 'Watch Lichess TV', 'Solve puzzles', 'Review games and studies', 'Learn chess', 'Configure ChessTUI']
  return (
    <div className="home-screen">
      <pre className="wordmark">{wordmark}</pre>
      <p className="muted">{account ? `${account.title ? `${account.title} ` : ''}${account.username}  •  connected to Lichess` : 'Offline mode'}</p>
      <div className="home-actions">
        {actions.map((label, index) => <button type="button" onClick={() => activate(index + 1)} key={label}><span>{index + 2}</span>{label}</button>)}
      </div>
      <p className="muted">0 puzzles&nbsp; • &nbsp;0 solved&nbsp; • &nbsp;streak 0&nbsp; • &nbsp;best 0</p>
      <p className="muted">Type :help or press ? for the keyboard map</p>
    </div>
  )
}

function PlayScreen({ registerKeys, token, account, authError, disconnect, setMode }: {
  registerKeys: RegisterKeys
  token: string
  account: Account | null
  authError: string
  disconnect: () => void
  setMode: (mode: InputMode) => void
}) {
  const [message, setMessage] = useState(account ? `✓ Connected as ${account.username}` : 'Choose how you want to play.')
  const [busy, setBusy] = useState(false)
  const [gameId, setGameId] = useState('')
  const [player, setPlayer] = useState<'white' | 'black'>('white')
  const [game, setGame] = useState<GameFullEvent | null>(null)
  const [state, setState] = useState<GameStateEvent | null>(null)
  const [fen, setFen] = useState('start')
  const [input, setInput] = useState('')
  const [promptUser, setPromptUser] = useState(false)
  const [boardCursor, setBoardCursor] = useState('e2')
  const [selectedSquare, setSelectedSquare] = useState('')
  const [legalSquares, setLegalSquares] = useState<string[]>([])
  const gameStream = useRef<AbortController | null>(null)
  const eventStream = useRef<AbortController | null>(null)
  const initialFen = useRef('startpos')
  const visibleAuthError = account ? '' : authError
  const connect = useCallback(() => { void beginLogin() }, [])

  const consumeState = useCallback((next: GameStateEvent) => {
    setState(next)
    try {
      setFen(replayGame(initialFen.current, next.moves))
    } catch {
      setMessage('Lichess sent a position this client could not replay.')
    }
    if (next.status !== 'started') {
      const result = next.winner ? `${next.winner} won` : 'draw'
      setMessage(`Game finished: ${next.status} • ${result}. Press b for another AI game.`)
      setBusy(false)
    } else {
      setMessage('Game connected. Use hjkl and Enter, or press i to type a move.')
    }
    setSelectedSquare('')
    setLegalSquares([])
  }, [])

  const openGame = useCallback((created: CreatedGame) => {
    const id = created.id
    if (!id) throw new Error('Lichess did not return a game ID.')
    gameStream.current?.abort()
    eventStream.current?.abort()
    setGameId(id)
    setGame(null)
    setState(null)
    setFen(created.fen || 'start')
    setPlayer(created.player || created.color || 'white')
    setInput('')
    setPromptUser(false)
    setSelectedSquare('')
    setLegalSquares([])
    setBusy(false)
    initialFen.current = 'startpos'
    const controller = new AbortController()
    gameStream.current = controller
    void streamBoardGame(token, id, controller.signal, (update: BoardGameEvent) => {
      if (update.type === 'gameFull') {
        const full = update as GameFullEvent
        setGame(full)
        initialFen.current = full.initialFen || 'startpos'
        if (account?.id && full.white.id === account.id) setPlayer('white')
        else if (account?.id && full.black.id === account.id) setPlayer('black')
        consumeState(full.state)
      } else if (update.type === 'gameState') consumeState(update as GameStateEvent)
    }).catch((error: unknown) => {
      if ((error as Error).name !== 'AbortError') setMessage(error instanceof Error ? error.message : String(error))
    })
  }, [account?.id, consumeState, token])

  const startAI = useCallback(async () => {
    if (!token) { setMessage('Connect your Lichess account first.'); return }
    setBusy(true)
    setMessage('Creating a casual 5+3 game against Lichess AI level 3…')
    try {
      openGame(await createAIGame(token, 3))
    } catch (error) {
      setBusy(false)
      setMessage(error instanceof Error ? error.message : String(error))
    }
  }, [openGame, token])

  const challengeUser = useCallback(async () => {
    const username = input.trim()
    if (!username || !token) return
    setBusy(true)
    setPromptUser(false)
    setInput('')
    setMode('nav')
    setMessage(`Sending a casual 5+3 challenge to ${username}…`)
    try {
      const challenge = await createUserChallenge(token, username)
      const challengeId = challenge.id || challenge.challenge?.id
      setMessage(`Challenge ${challengeId || ''} sent to ${username}. Waiting for acceptance…`)
      const controller = new AbortController()
      eventStream.current?.abort()
      eventStream.current = controller
      void streamUserEvents(token, controller.signal, (event) => {
        if (event.type !== 'gameStart' || !event.game) return
        const id = event.game.id || event.game.gameId
        if (!id) return
        openGame({ id, player: event.game.color })
      }).catch((error: unknown) => {
        if ((error as Error).name !== 'AbortError') setMessage(error instanceof Error ? error.message : String(error))
      })
    } catch (error) {
      setBusy(false)
      setMessage(error instanceof Error ? error.message : String(error))
    }
  }, [input, openGame, setMode, token])

  const sendMove = useCallback(async (move: string) => {
    if (!gameId || !token) return
    setBusy(true)
    try {
      await makeBoardMove(token, gameId, move)
      setInput('')
      setSelectedSquare('')
      setLegalSquares([])
      setBoardCursor(move.slice(2, 4))
      setMode('nav')
      setMessage('Move sent. Waiting for Lichess…')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
    }
  }, [gameId, setMode, token])

  const submitMove = useCallback(async () => {
    if (!input.trim()) return
    try {
      await sendMove(resolveMoveInput(fen, input))
    } catch (error) {
      const detail = error instanceof Error ? error.message : 'Move could not be parsed.'
      setMessage(`${input.trim()} is not legal in this position. ${detail}`)
      setInput('')
    }
  }, [fen, input, sendMove])

  const activateBoardCursor = useCallback(() => {
    if (!gameId || busy) return
    const position = new Chess(fen === 'start' ? undefined : fen)
    const sideToMove = position.turn() === 'w' ? 'white' : 'black'
    if (sideToMove !== player) {
      setMessage('Wait for the opponent to move.')
      return
    }
    if (!selectedSquare) {
      const targets = legalTargets(fen, boardCursor)
      if (targets.length === 0) {
        setMessage(`No movable ${player} piece on ${boardCursor}.`)
        return
      }
      setSelectedSquare(boardCursor)
      setLegalSquares(targets)
      setMessage(`${boardCursor} selected. Move the cursor to a highlighted square and press Enter.`)
      return
    }
    if (boardCursor === selectedSquare) {
      setSelectedSquare('')
      setLegalSquares([])
      setMessage('Selection cancelled.')
      return
    }
    try {
      void sendMove(resolveBoardDrop(fen, selectedSquare, boardCursor))
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    }
  }, [boardCursor, busy, fen, gameId, player, selectedSquare, sendMove])

  const finishGame = useCallback(async () => {
    if (!gameId || !token) return
    const action = state?.moves ? 'resign' : 'abort'
    setBusy(true)
    try {
      await endBoardGame(token, gameId, action)
      setMessage(action === 'abort' ? 'Game aborted.' : 'Game resigned.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
    }
  }, [gameId, state?.moves, token])

  useEffect(() => {
    setMessage(account ? `✓ Connected as ${account.username}` : 'Choose how you want to play.')
  }, [account])
  useEffect(() => {
    setBoardCursor(player === 'white' ? 'e2' : 'e7')
    setSelectedSquare('')
    setLegalSquares([])
  }, [gameId, player])
  useEffect(() => {
    if (!account || !token || gameId) return
    let cancelled = false
    void currentGames(token).then(({ nowPlaying }) => {
      if (!cancelled && nowPlaying[0]) openGame({ id: nowPlaying[0].gameId, player: nowPlaying[0].color, fen: nowPlaying[0].fen })
    }).catch((error: unknown) => {
      if (!cancelled) setMessage(error instanceof Error ? error.message : String(error))
    })
    return () => { cancelled = true }
  }, [account, gameId, openGame, token])
  useEffect(() => () => {
    gameStream.current?.abort()
    eventStream.current?.abort()
  }, [])

  useEffect(() => registerKeys((event, inputMode) => {
    if (inputMode === 'nav') {
      if (gameId && ['h', 'j', 'k', 'l', 'ArrowLeft', 'ArrowDown', 'ArrowUp', 'ArrowRight'].includes(event.key)) {
        setBoardCursor((square) => moveBoardCursor(square, event.key, player))
        return true
      }
      if (gameId && event.key === 'Enter') { activateBoardCursor(); return true }
      if (gameId && event.key === 'Escape' && selectedSquare) {
        setSelectedSquare(''); setLegalSquares([]); setMessage('Selection cancelled.'); return true
      }
      if (event.key === 'a' && !account) { connect(); return true }
      if (event.key === 'b' && account && !busy) {
        if (gameId && state?.status === 'started') setMessage('Finish or resign the current game with x before starting another.')
        else void startAI()
        return true
      }
      if (event.key === 'u' && account && !busy) { setPromptUser(true); setInput(''); setMessage('Enter a Lichess username, then press Enter.'); setMode('insert'); return true }
      if (event.key === 'x' && gameId && !busy && state?.status === 'started') { void finishGame(); return true }
      if (event.key === 'd' && account && !gameId) { disconnect(); setMessage('Disconnected from Lichess.'); return true }
      if (['b', 'u'].includes(event.key) && !account) { setMessage('Connect your Lichess account first with a.'); return true }
      return false
    }
    if (!promptUser && !gameId) return false
    if (event.key === 'Enter') { void (promptUser ? challengeUser() : submitMove()); return true }
    if (event.key === 'Backspace') { setInput((value) => value.slice(0, -1)); return true }
    if (event.key.length === 1) { setInput((value) => value + event.key); return true }
    return false
  }), [account, activateBoardCursor, busy, challengeUser, connect, disconnect, finishGame, gameId, player, promptUser, registerKeys, selectedSquare, setMode, startAI, state?.status, submitMove])

  if (gameId) {
    const yourTurn = state ? (state.moves.trim().split(/\s+/).filter(Boolean).length % 2 === 0 ? 'white' : 'black') === player : false
    return (
      <div className="board-screen play-board">
        <ChessBoard fen={fen} orientation={player} label={`Lichess game ${gameId}`} cursor={boardCursor} selected={selectedSquare} legalMoves={legalSquares} />
        <div className="board-copy">
          <h1>{game?.white.name || 'White'} vs {game?.black.name || 'Black'}</h1>
          <p className="muted">Game {gameId}&nbsp; • &nbsp;you are {player}<br />{formatClock(state?.wtime)} / {formatClock(state?.btime)}&nbsp; • &nbsp;{state?.status || 'connecting'}</p>
          <p>{message}</p>
          {state?.status === 'started' ? <><p className={yourTurn ? 'success' : 'muted'}>{yourTurn ? 'Your move.' : 'Waiting for opponent…'}</p><p>cursor › {boardCursor}{selectedSquare ? `  •  selected ${selectedSquare}` : ''}</p><p>move › {input}_</p><p className="muted">hjkl cursor • enter select/drop • esc cancel • i type • x {state.moves ? 'resign' : 'abort'}</p></> : <p className="muted">b new AI game</p>}
        </div>
      </div>
    )
  }

  return (
    <div className="plain-screen">
      <h1>Play</h1>
      <div className="menu-list">
        {!account ? <p><span>a</span>Connect Lichess account</p> : null}
        {account ? <><p><span>b</span>Play Lichess AI level 3</p><p><span>u</span>Challenge a Lichess user</p></> : null}
        {account ? <p><span>d</span>Disconnect {account.username}</p> : null}
      </div>
      {promptUser ? <p>username › {input}_</p> : null}
      <p className={visibleAuthError ? 'error' : 'muted'}>{visibleAuthError || message}</p>
    </div>
  )
}

function replayGame(initialFen: string, moves: string) {
  const position = new Chess(initialFen && initialFen !== 'startpos' ? initialFen : undefined)
  for (const move of moves.trim().split(/\s+/).filter(Boolean)) {
    position.move({ from: move.slice(0, 2), to: move.slice(2, 4), promotion: move.slice(4, 5) || 'q' })
  }
  return position.fen()
}

function formatClock(milliseconds?: number) {
  if (milliseconds == null) return '--:--'
  const seconds = Math.max(0, Math.ceil(milliseconds / 1000))
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`
}

function PuzzleScreen({ registerKeys, token }: { registerKeys: RegisterKeys; token: string }) {
  const [puzzle, setPuzzle] = useState<PuzzlePayload | null>(null)
  const [fen, setFen] = useState('start')
  const [input, setInput] = useState('')
  const [step, setStep] = useState(0)
  const [message, setMessage] = useState('Loading puzzle…')

  const load = useCallback(async (kind: 'daily' | 'next') => {
    setMessage('Loading puzzle…')
    try {
      const value = kind === 'daily' ? await dailyPuzzle() : await nextPuzzle(token || undefined)
      setPuzzle(value); setFen(value.puzzle.fen); setStep(0); setInput(''); setMessage('Find the best move.')
    } catch (error) { setMessage(error instanceof Error ? error.message : String(error)) }
  }, [token])
  useEffect(() => { void load('daily') }, [load])

  const submit = useCallback(() => {
    if (!puzzle || !input) return
    const expected = puzzle.puzzle.solution[step]?.toLowerCase()
    if (input.toLowerCase() !== expected) { setMessage('That move misses the tactic. Try another line.'); setInput(''); return }
    try {
      const game = new Chess(fen)
      game.move({ from: input.slice(0, 2), to: input.slice(2, 4), promotion: input.slice(4, 5) || 'q' })
      let nextStep = step + 1
      if (nextStep < puzzle.puzzle.solution.length) {
        const reply = puzzle.puzzle.solution[nextStep]
        game.move({ from: reply.slice(0, 2), to: reply.slice(2, 4), promotion: reply.slice(4, 5) || 'q' })
        nextStep++
      }
      setFen(game.fen()); setStep(nextStep); setInput('')
      setMessage(nextStep < puzzle.puzzle.solution.length ? '✓ Correct. Continue the combination.' : '✓ Puzzle solved — press n for another.')
    } catch { setMessage('That move is not legal in this position.'); setInput('') }
  }, [fen, input, puzzle, step])

  useEffect(() => registerKeys((event, inputMode) => {
    if (inputMode === 'nav') {
      if (event.key === 'd') { void load('daily'); return true }
      if (event.key === 'n') { void load('next'); return true }
      if (event.key === 'h' && puzzle) { setMessage(`Hint: move the piece on ${puzzle.puzzle.solution[step]?.slice(0, 2)}`); return true }
      return false
    }
    if (event.key === 'Enter') { submit(); return true }
    if (event.key === 'Backspace') { setInput((value) => value.slice(0, -1)); return true }
    if (event.key.length === 1) { setInput((value) => value + event.key.toLowerCase()); return true }
    return false
  }), [input, load, puzzle, registerKeys, step, submit])

  return (
    <div className="board-screen puzzle-screen">
      <ChessBoard fen={fen} label={puzzle ? `Lichess puzzle ${puzzle.puzzle.id}` : 'Loading puzzle'} />
      <div className="board-copy">
        <h1>Puzzle trainer</h1>
        {puzzle ? <p className="muted">Puzzle {puzzle.puzzle.id}&nbsp; • &nbsp;rating {puzzle.puzzle.rating}<br />{puzzle.puzzle.themes.join(' • ')}</p> : null}
        <p>{message}</p>
        <p>move › {input}_</p>
      </div>
    </div>
  )
}

function WatchScreen({ registerKeys }: { registerKeys: RegisterKeys }) {
  const [section, setSection] = useState(0)
  const [selected, setSelected] = useState([0, 0, 0])
  const [channels, setChannels] = useState<Array<[string, TVChannel]>>([])
  const [streamers, setStreamers] = useState<Streamer[]>([])
  const [broadcasts, setBroadcasts] = useState<Broadcast[]>([])
  const [event, setEvent] = useState<TVEvent | null>(null)
  const [message, setMessage] = useState('Refreshing Lichess…')
  const controller = useRef<AbortController | null>(null)

  const refresh = useCallback(async () => {
    setMessage('Refreshing Lichess…')
    try {
      const data = await watchDirectory()
      setChannels(Object.entries(data.channels).sort(([a], [b]) => a.localeCompare(b)))
      setStreamers(data.streamers); setBroadcasts(data.broadcasts); setMessage('')
    } catch (error) { setMessage(error instanceof Error ? error.message : String(error)) }
  }, [])
  useEffect(() => { void refresh(); return () => controller.current?.abort() }, [refresh])
  const lists = [channels, streamers, broadcasts]

  const activateItem = useCallback(() => {
    const index = selected[section]
    if (section === 0 && channels[index]) {
      controller.current?.abort()
      controller.current = new AbortController()
      const channel = channels[index][0]
      setMessage(`Opening Lichess TV • ${channel}…`)
      void streamTV(channel, controller.current.signal, (update) => { setEvent(update); setMessage('') })
        .catch((error: unknown) => { if ((error as Error).name !== 'AbortError') setMessage(error instanceof Error ? error.message : String(error)) })
    } else if (section === 1 && streamers[index]) {
      const item = streamers[index]
      window.open(item.stream.service === 'youtube' ? item.streamer.youtube : item.streamer.twitch, '_blank', 'noopener')
    } else if (section === 2 && broadcasts[index]) window.open(broadcasts[index].tour.url, '_blank', 'noopener')
  }, [broadcasts, channels, section, selected, streamers])

  useEffect(() => registerKeys((keyEvent, inputMode) => {
    if (inputMode === 'insert') return false
    if (keyEvent.key === ']' || keyEvent.key === 'Tab') { setSection((value) => (value + 1) % 3); return true }
    if (keyEvent.key === '[') { setSection((value) => (value + 2) % 3); return true }
    if (keyEvent.key === 'j' || keyEvent.key === 'ArrowDown') {
      setSelected((value) => value.map((item, index) => index === section ? Math.min(item + 1, Math.max(0, lists[section].length - 1)) : item)); return true
    }
    if (keyEvent.key === 'k' || keyEvent.key === 'ArrowUp') {
      setSelected((value) => value.map((item, index) => index === section ? Math.max(0, item - 1) : item)); return true
    }
    if (keyEvent.key === 'Enter') { activateItem(); return true }
    if (keyEvent.key === 'r') { void refresh(); return true }
    return false
  }), [activateItem, lists, refresh, registerKeys, section])

  const rows = section === 0
    ? channels.map(([key, item]) => `${key.padEnd(11)}  ${item.user.name.padEnd(18)} ${item.rating}`)
    : section === 1
      ? streamers.map((item) => `${item.streamer.name.padEnd(18)} ${item.stream.service.padEnd(8)} ${item.stream.status}`)
      : broadcasts.map((item) => `${item.round.ongoing ? '● LIVE ' : ''}${item.tour.name}`)

  return (
    <div className="watch-screen">
      <div className="watch-list">
        <div className="subtabs">{['TV games', 'Live streams', 'Broadcasts'].map((label, index) => <button type="button" className={section === index ? 'active' : ''} onClick={() => setSection(index)} key={label}>{label}</button>)}</div>
        <div className="rows">{rows.slice(0, 13).map((row, index) => <p className={selected[section] === index ? 'selected' : ''} key={row}> {selected[section] === index ? '›' : ' '} {row}</p>)}</div>
        {message ? <p className="muted">{message}</p> : null}
      </div>
      <div className="tv-board">
        {event?.d.fen ? <><h1>Lichess TV</h1><ChessBoard fen={event.d.fen} orientation={event.d.orientation || 'white'} /></> : <div className="empty-panel">Choose a TV channel to watch it natively.<br /><br />Live stream videos and broadcast pages open in your browser.</div>}
      </div>
    </div>
  )
}

const lessons = [
  { title: 'How pieces move', description: 'Move the white pawn two squares to claim the center.', fen: '8/8/8/8/8/8/4P3/4K2k w - - 0 1', goal: 'Enter e2e4', move: 'e2e4' },
  { title: 'Develop with purpose', description: 'Bring the knight toward the center where it controls four central squares.', fen: 'rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1', goal: 'Enter g8f6', move: 'g8f6' },
  { title: 'Castle to safety', description: 'Complete kingside castling. The king and rook move together.', fen: 'r3k2r/pppq1ppp/2npbn2/8/8/2NPBN2/PPPQ1PPP/R3K2R w KQkq - 0 1', goal: 'Enter e1g1', move: 'e1g1' },
  { title: 'Spot checkmate', description: 'The queen is protected by the king. Deliver mate on g7.', fen: '7k/5K2/8/6Q1/8/8/8/8 w - - 0 1', goal: 'Enter g5g7', move: 'g5g7' },
]

function LearnScreen({ registerKeys }: { registerKeys: RegisterKeys }) {
  const [selected, setSelected] = useState(0)
  const [input, setInput] = useState('')
  const [fen, setFen] = useState(lessons[0].fen)
  const [message, setMessage] = useState('')
  const lesson = lessons[selected]
  const choose = useCallback((index: number) => { setSelected(index); setFen(lessons[index].fen); setInput(''); setMessage('') }, [])
  const submit = useCallback(() => {
    if (input !== lesson.move) { setMessage('Not quite — look at the goal and try again.'); setInput(''); return }
    const game = new Chess(fen)
    game.move({ from: input.slice(0, 2), to: input.slice(2, 4), promotion: 'q' })
    setFen(game.fen()); setInput(''); setMessage('✓ Lesson complete. Choose the next lesson with ↓.')
  }, [fen, input, lesson.move])
  useEffect(() => registerKeys((event, inputMode) => {
    if (inputMode === 'nav') {
      if (event.key === 'j' || event.key === 'ArrowDown') { choose(Math.min(3, selected + 1)); return true }
      if (event.key === 'k' || event.key === 'ArrowUp') { choose(Math.max(0, selected - 1)); return true }
      return false
    }
    if (event.key === 'Enter') { submit(); return true }
    if (event.key === 'Backspace') { setInput((value) => value.slice(0, -1)); return true }
    if (event.key.length === 1) { setInput((value) => value + event.key.toLowerCase()); return true }
    return false
  }), [choose, input, registerKeys, selected, submit])
  return (
    <div className="learn-screen">
      <div className="lesson-list"><p>Learn chess at your own pace</p>{lessons.map((item, index) => <button type="button" className={selected === index ? 'selected' : ''} onClick={() => choose(index)} key={item.title}>{selected === index ? '› ' : '  '}{item.title}</button>)}</div>
      <div className="lesson-detail">
        <ChessBoard fen={fen} label={lesson.title} />
        <div><h1>{lesson.title}</h1><p>{lesson.description}</p><p>{lesson.goal}</p><p>move › {input}_</p>{message ? <p className={message.startsWith('✓') ? 'success' : 'error'}>{message}</p> : null}</div>
      </div>
    </div>
  )
}

function StudyScreen({ registerKeys, account }: { registerKeys: RegisterKeys; account: Account | null }) {
  const [selected, setSelected] = useState(0)
  const username = account?.username || ''
  const items = [
    { label: 'Recent games', url: `https://lichess.org/@/${encodeURIComponent(username)}/games` },
    { label: 'Lichess studies', url: `https://lichess.org/study/by/${encodeURIComponent(username)}` },
    { label: 'Analysis board', url: 'https://lichess.org/analysis' },
  ]
  const open = useCallback(() => {
    if (!account && selected < 2) return
    window.open(items[selected].url, '_blank', 'noopener')
  }, [account, items, selected])
  useEffect(() => registerKeys((event, inputMode) => {
    if (inputMode === 'insert') return false
    if (event.key === 'j' || event.key === 'ArrowDown') { setSelected((value) => Math.min(items.length - 1, value + 1)); return true }
    if (event.key === 'k' || event.key === 'ArrowUp') { setSelected((value) => Math.max(0, value - 1)); return true }
    if (event.key === 'Enter') { open(); return true }
    return false
  }), [items.length, open, registerKeys])
  return <div className="plain-screen"><h1>Study</h1><div className="menu-list">{items.map((item, index) => <p className={selected === index ? 'selected' : ''} key={item.label}><span>{selected === index ? '›' : ' '}</span>{item.label}</p>)}</div><p className="muted">{account ? `Connected as ${account.username}. Press Enter to open the selected Lichess view.` : 'Connect Lichess from Play for account games and studies. The analysis board is public.'}</p></div>
}

function SettingsScreen() {
  return <div className="plain-screen"><h1>Settings</h1><div className="settings-list"><p>Piece style <span>SVG images</span></p><p>Show coordinates <span>yes</span></p><p>Move input <span>UCI text</span></p><p>Default TV channel <span>best</span></p><p>Stockfish <span>native app only</span></p></div><p className="muted"><a href="https://github.com/kannachi323/chess-cli" target="_blank" rel="noreferrer">GitHub repository</a>&nbsp; • &nbsp;<a href="https://github.com/kannachi323" target="_blank" rel="noreferrer">@kannachi323</a></p></div>
}

function HelpScreen() {
  return <div className="help-screen"><h1>ChessTUI keyboard map</h1><div><h2>NAV MODE</h2><p>h j k l&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; move in the focused region or across the board</p><p>enter&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; select a piece / drop on a legal square</p><p>esc&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; cancel a piece selection</p><p>i&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; insert UCI, SAN, or a resulting FEN</p><p>tab / shift-tab&nbsp; alternate navbar and content</p><p>ctrl-w k / j&nbsp;&nbsp;&nbsp; focus navbar / content</p><p>gt / gT&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; next / previous workspace</p><p>H / L&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; previous / next workspace</p><p>:&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; enter a command</p></div><div><h2>INSERT MODE</h2><p>all printable keys&nbsp; type into the active field</p><p>backspace&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; delete the previous character</p><p>enter&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; submit the field</p><p>esc&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; return to NAV</p></div><div><h2>COMMANDS</h2><p>:home :play :watch :puzzles :study :learn :settings</p><p>:tn :tp :help :nav :content :q</p></div><p className="muted">There are only two persistent modes: NAV and INSERT.</p></div>
}
