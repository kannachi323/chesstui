import { useEffect, useRef, useState } from 'react'

export const REPO_URL = 'https://github.com/kannachi323/chesstui'
export const RELEASES_URL = `${REPO_URL}/releases`
// Switch to `brew install kannachi323/tap/chesstui` once the tap exists.
export const INSTALL_COMMAND = 'git clone https://github.com/kannachi323/chesstui && make -C chesstui build'

const HERO_FRAMES = Array.from({ length: 16 }, (_, i) => `/shots/hero/${String(i).padStart(2, '0')}.svg`)

const FEATURES = [
  { src: '/shots/analyze-card.svg', title: 'Analyze', caption: 'Your engine reviews every move and marks the mistakes.' },
  { src: '/shots/read-card.svg', title: 'Read', caption: 'Chess books in a terminal split. The board follows the text.' },
  { src: '/shots/play-card.svg', title: 'Play', caption: 'Stockfish or any UCI engine, saved and resumable.' },
  { src: '/shots/learn-light-card.svg', title: 'Adapts', caption: 'Light or dark, it follows your terminal.' },
]

function prefersReducedMotion() {
  return typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
}

/** Replays frames recorded from the real reader, one move per beat. */
export function HeroTerminal({ interval = 1100 }: { interval?: number }) {
  const [frame, setFrame] = useState(() => (prefersReducedMotion() ? HERO_FRAMES.length - 1 : 0))

  useEffect(() => {
    if (prefersReducedMotion()) return
    let timer: number
    const advance = (current: number) => {
      const next = (current + 1) % HERO_FRAMES.length
      // Linger on the last position before starting over.
      const wait = next === 0 ? interval * 3 : interval
      timer = window.setTimeout(() => {
        setFrame(next)
        advance(next)
      }, wait)
    }
    advance(frame)
    return () => window.clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [interval])

  return (
    <figure className="hero-terminal" aria-label="chesstui reading a chess book, with the board following the moves">
      {HERO_FRAMES.map((src, i) => (
        <img
          key={src}
          src={src}
          alt=""
          className={i === frame ? 'frame is-visible' : 'frame'}
          width={1047}
          height={622}
          loading={i < 2 ? 'eager' : 'lazy'}
          decoding="async"
          data-testid="hero-frame"
        />
      ))}
    </figure>
  )
}

export function InstallCommand({ command = INSTALL_COMMAND }: { command?: string }) {
  const [copied, setCopied] = useState(false)
  const codeRef = useRef<HTMLElement>(null)

  async function copy() {
    try {
      await navigator.clipboard.writeText(command)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
    } catch {
      // Clipboard blocked: select the text so a manual copy is one keystroke.
      const selection = window.getSelection()
      if (selection && codeRef.current) {
        const range = document.createRange()
        range.selectNodeContents(codeRef.current)
        selection.removeAllRanges()
        selection.addRange(range)
      }
    }
  }

  return (
    <div className="install">
      <span className="prompt" aria-hidden="true">$</span>
      <code ref={codeRef}>
        {command.split(' && ').map((part, i, parts) => (
          <span key={part} className="cmd-part">
            {/* Allow line breaks only after slashes, never inside a word. */}
            {part.split('/').map((piece, j, pieces) => (
              <span key={j}>
                {piece}
                {j < pieces.length - 1 ? (
                  <>
                    /<wbr />
                  </>
                ) : null}
              </span>
            ))}
            {i < parts.length - 1 ? ' && ' : ''}
          </span>
        ))}
      </code>
      <button type="button" onClick={copy} aria-label={copied ? 'Copied' : 'Copy install command'}>
        {copied ? <CheckIcon /> : <CopyIcon />}
      </button>
      <span className="visually-hidden" aria-live="polite">
        {copied ? 'Copied to clipboard' : ''}
      </span>
    </div>
  )
}

export default function App() {
  return (
    <>
      <header className="nav">
        <a className="wordmark" href="/" aria-label="chesstui home">
          <span className="knight" aria-hidden="true">♞</span>chesstui
        </a>
        <a className="nav-link" href={REPO_URL}>
          <GitHubIcon />
          <span>GitHub</span>
        </a>
      </header>

      <main>
        <section className="hero">
          <h1>Chess in the terminal.</h1>
          <p className="lede">Play your engine, review your games and read chess books, right in your terminal.</p>
          <InstallCommand />
          <p className="alt">
            or <a href={RELEASES_URL}>download for macOS, Linux and Windows</a>
          </p>
          <HeroTerminal />
        </section>

        <section className="features" aria-label="Features">
          {FEATURES.map((feature) => (
            <figure key={feature.title} className="feature">
              <img src={feature.src} alt={`chesstui ${feature.title.toLowerCase()} screen`} loading="lazy" decoding="async" />
              <figcaption>
                <strong>{feature.title}</strong> {feature.caption}
              </figcaption>
            </figure>
          ))}
        </section>

        <section className="keys" aria-label="Keyboard">
          <div className="keycaps" aria-hidden="true">
            <kbd>e4</kbd>
            <kbd>Nf3</kbd>
            <kbd>O-O</kbd>
            <kbd>←</kbd>
            <kbd>↵</kbd>
          </div>
          <p>Type moves, or walk the board with the arrows. Everything stays on your machine.</p>
        </section>
      </main>

      <footer className="footer">
        <span>Apache-2.0 · bring your own UCI engine · not affiliated with Lichess</span>
        <a href={REPO_URL}>Source</a>
      </footer>
    </>
  )
}

function GitHubIcon() {
  return (
    <svg viewBox="0 0 16 16" width="18" height="18" aria-hidden="true">
      <path
        fill="currentColor"
        d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"
      />
    </svg>
  )
}

function CopyIcon() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8">
      <rect x="9" y="9" width="11" height="11" rx="2" />
      <path d="M5 15V5a2 2 0 0 1 2-2h8" />
    </svg>
  )
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M5 12.5l4.5 4.5L19 7.5" />
    </svg>
  )
}
