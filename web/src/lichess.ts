const LICHESS = 'https://lichess.org'
const OAUTH_KEY = 'chesstui.oauth'
const TOKEN_KEY = 'chesstui.token'
let loginPromise: Promise<string> | null = null

export type Account = {
  id: string
  username: string
  title?: string
}

export type PuzzlePayload = {
  game: { id: string; pgn: string }
  puzzle: {
    id: string
    rating: number
    plays: number
    solution: string[]
    themes: string[]
    fen: string
    lastMove: string
  }
}

export type TVChannel = {
  user: { id: string; name: string; title?: string }
  rating: number
  gameId: string
  color: string
}

export type Streamer = {
  id: string
  name: string
  title?: string
  stream: { service: string; status: string; lang: string }
  streamer: { name: string; twitch?: string; youtube?: string }
}

export type Broadcast = {
  tour: { id: string; name: string; url: string }
  round: { ongoing: boolean }
}

export type TVEvent = {
  t: string
  d: {
    id: string
    orientation: 'white' | 'black'
    fen: string
    lm: string
    players?: Array<{
      color: string
      user: { name: string; title?: string }
      rating: number
    }>
  }
}

export type GameStateEvent = {
  type: 'gameState'
  moves: string
  wtime: number
  btime: number
  winc: number
  binc: number
  status: string
  winner?: 'white' | 'black'
}

export type GamePlayer = {
  id?: string
  name: string
  title?: string
  rating?: number
}

export type GameFullEvent = {
  type: 'gameFull'
  id: string
  initialFen: string
  white: GamePlayer
  black: GamePlayer
  state: GameStateEvent
}

export type BoardGameEvent = GameFullEvent | GameStateEvent | { type: string; [key: string]: unknown }

export type CreatedGame = {
  id: string
  fullId?: string
  fen?: string
  player?: 'white' | 'black'
  color?: 'white' | 'black'
}

export type UserEvent = {
  type: string
  game?: { id?: string; gameId?: string; color?: 'white' | 'black'; opponent?: { username?: string } }
}

export type PlayingGame = {
  gameId: string
  color: 'white' | 'black'
  fen: string
  opponent?: { username?: string }
}

async function api<T>(path: string, token?: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const response = await fetch(`${LICHESS}${path}`, { ...init, headers })
  if (!response.ok) {
    const message = (await response.text()).trim() || response.statusText
    throw new Error(`Lichess returned ${response.status}: ${message}`)
  }
  return response.json() as Promise<T>
}

export function dailyPuzzle() {
  return api<PuzzlePayload>('/api/puzzle/daily')
}

export function nextPuzzle(token?: string) {
  return api<PuzzlePayload>('/api/puzzle/next', token)
}

export async function watchDirectory() {
  const channels = await api<Record<string, TVChannel>>('/api/tv/channels')
  const streamers = await api<Streamer[]>('/api/streamer/live')
  const broadcasts = await api<{ active: Broadcast[] }>('/api/broadcast/top?page=1')
  return { channels, streamers, broadcasts: broadcasts.active }
}

export async function streamTV(channel: string, signal: AbortSignal, update: (event: TVEvent) => void) {
  const response = await fetch(`${LICHESS}/api/tv/${encodeURIComponent(channel)}/feed`, {
    headers: { Accept: 'application/x-ndjson' },
    signal,
  })
  if (!response.ok || !response.body) throw new Error(`Lichess TV returned ${response.status}`)
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''
    for (const line of lines) {
      if (line.trim()) update(JSON.parse(line) as TVEvent)
    }
  }
}

export function storedToken() {
  return sessionStorage.getItem(TOKEN_KEY) ?? ''
}

export function clearToken() {
  sessionStorage.removeItem(TOKEN_KEY)
}

export function account(token: string) {
  return api<Account>('/api/account', token)
}

export function currentGames(token: string) {
  return api<{ nowPlaying: PlayingGame[] }>('/api/account/playing', token)
}

export function createAIGame(token: string, level = 3) {
  return api<CreatedGame>('/api/challenge/ai', token, {
    method: 'POST',
    body: new URLSearchParams({
      level: String(level),
      'clock.limit': '300',
      'clock.increment': '3',
      color: 'random',
      variant: 'standard',
    }),
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  })
}

export function createUserChallenge(token: string, username: string) {
  return api<{ id?: string; challenge?: { id?: string } }>(`/api/challenge/${encodeURIComponent(username)}`, token, {
    method: 'POST',
    body: new URLSearchParams({
      rated: 'false',
      'clock.limit': '300',
      'clock.increment': '3',
      color: 'random',
      variant: 'standard',
    }),
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  })
}

export function makeBoardMove(token: string, gameId: string, move: string) {
  return api<{ ok: boolean }>(`/api/board/game/${encodeURIComponent(gameId)}/move/${encodeURIComponent(move)}`, token, { method: 'POST' })
}

export function endBoardGame(token: string, gameId: string, action: 'abort' | 'resign') {
  return api<{ ok: boolean }>(`/api/board/game/${encodeURIComponent(gameId)}/${action}`, token, { method: 'POST' })
}

export function streamBoardGame(token: string, gameId: string, signal: AbortSignal, update: (event: BoardGameEvent) => void) {
  return streamNDJSON(`/api/board/game/stream/${encodeURIComponent(gameId)}`, token, signal, update)
}

export function streamUserEvents(token: string, signal: AbortSignal, update: (event: UserEvent) => void) {
  return streamNDJSON('/api/stream/event', token, signal, update)
}

export async function beginLogin() {
  const verifier = randomToken(64)
  const state = randomToken(32)
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))
  const challenge = base64url(new Uint8Array(digest))
  const redirectUri = `${location.origin}${location.pathname}`
  sessionStorage.setItem(OAUTH_KEY, JSON.stringify({ verifier, state, redirectUri }))
  const query = new URLSearchParams({
    response_type: 'code',
    client_id: 'chesstui-web',
    redirect_uri: redirectUri,
    code_challenge_method: 'S256',
    code_challenge: challenge,
    state,
    scope: 'board:play challenge:read challenge:write study:read puzzle:read preference:read',
  })
  location.assign(`${LICHESS}/oauth?${query}`)
}

export function finishLogin(): Promise<string> {
  // React Strict Mode replays mount effects in development. Authorization codes
  // are single-use, so every caller must share the same token exchange.
  loginPromise ??= finishLoginOnce()
  return loginPromise
}

async function finishLoginOnce(): Promise<string> {
  const query = new URLSearchParams(location.search)
  const code = query.get('code')
  if (!code) return storedToken()
  const saved = sessionStorage.getItem(OAUTH_KEY)
  if (!saved) throw new Error('The Lichess login session expired. Please try again.')
  const flow = JSON.parse(saved) as { verifier: string; state: string; redirectUri: string }
  if (query.get('state') !== flow.state) throw new Error('The Lichess login state did not match.')
  const body = new URLSearchParams({
    grant_type: 'authorization_code',
    code,
    code_verifier: flow.verifier,
    redirect_uri: flow.redirectUri,
    client_id: 'chesstui-web',
  })
  const result = await api<{ access_token: string }>('/api/token', undefined, {
    method: 'POST',
    body,
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  })
  sessionStorage.removeItem(OAUTH_KEY)
  sessionStorage.setItem(TOKEN_KEY, result.access_token)
  history.replaceState(null, '', flow.redirectUri)
  return result.access_token
}

function randomToken(size: number) {
  const bytes = crypto.getRandomValues(new Uint8Array(size))
  return base64url(bytes)
}

function base64url(bytes: Uint8Array) {
  let value = ''
  for (const byte of bytes) value += String.fromCharCode(byte)
  return btoa(value).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '')
}

async function streamNDJSON<T>(path: string, token: string, signal: AbortSignal, update: (event: T) => void) {
  const response = await fetch(`${LICHESS}${path}`, {
    headers: { Accept: 'application/x-ndjson', Authorization: `Bearer ${token}` },
    signal,
  })
  if (!response.ok || !response.body) {
    const message = (await response.text()).trim() || response.statusText
    throw new Error(`Lichess returned ${response.status}: ${message}`)
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''
    for (const line of lines) {
      if (line.trim()) update(JSON.parse(line) as T)
    }
  }
  if (buffer.trim()) update(JSON.parse(buffer) as T)
}
