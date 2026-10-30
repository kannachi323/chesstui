import { useEffect, useRef, useState } from 'react'
import '@xterm/xterm/css/xterm.css'

// Built by `make wasm`; absent in a site build without Go.
export const DEMO_WASM = '/demo/chesstui.wasm'
const DEMO_RUNTIME = '/demo/wasm_exec.js'

// Matches the recorded hero frames, so the switch to the live app is seamless.
const BACKGROUND = '#111214'
const BASE_FONT = 14
const BASE_WIDTH = 1047

type Bridge = {
  cols: number
  rows: number
  dark: boolean
  write: (bytes: Uint8Array) => void
  load: (key: string) => string | null
  save: (key: string, value: string) => void
  exit: (message: string) => void
  input?: (text: string) => void
  resize?: (cols: number, rows: number) => void
  stop?: () => void
}

declare global {
  interface Window {
    chesstui?: Bridge
    Go?: new () => { importObject: WebAssembly.Imports; run: (instance: WebAssembly.Instance) => Promise<void> }
  }
}

function loadRuntime(): Promise<void> {
  if (window.Go) return Promise.resolve()
  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = DEMO_RUNTIME
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('runtime'))
    document.head.appendChild(script)
  })
}

async function instantiate(imports: WebAssembly.Imports) {
  const response = await fetch(DEMO_WASM)
  if (!response.ok) throw new Error('missing')
  if (WebAssembly.instantiateStreaming && response.headers.get('content-type')?.includes('wasm')) {
    return (await WebAssembly.instantiateStreaming(response, imports)).instance
  }
  return (await WebAssembly.instantiate(await response.arrayBuffer(), imports)).instance
}

type State = 'loading' | 'running' | 'ended' | 'failed'

/** chesstui itself, compiled to WebAssembly and running in xterm.js. */
export default function LiveTerminal({ onClose }: { onClose: () => void }) {
  const host = useRef<HTMLDivElement>(null)
  const [state, setState] = useState<State>('loading')
  const [run, setRun] = useState(0)

  useEffect(() => {
    let disposed = false
    const cleanups: Array<() => void> = []

    async function start() {
      const [{ Terminal }, { FitAddon }, { ClipboardAddon }, { WebglAddon }] = await Promise.all([
        import('@xterm/xterm'),
        import('@xterm/addon-fit'),
        import('@xterm/addon-clipboard'),
        import('@xterm/addon-webgl'),
      ])
      if (disposed || !host.current) return
      const element = host.current
      const fontSize = () => Math.max(9, Math.min(BASE_FONT, (BASE_FONT * element.clientWidth) / BASE_WIDTH))
      const term = new Terminal({
        fontFamily: "'JetBrains Mono', 'SF Mono', Menlo, Consolas, 'DejaVu Sans Mono', monospace",
        fontSize: fontSize(),
        lineHeight: 1.1,
        // A real tty turns "\n" into "\r\n" (ONLCR); Bubble Tea relies on it.
        convertEol: true,
        cursorBlink: false,
        cursorStyle: 'bar',
        allowProposedApi: true,
        theme: { background: BACKGROUND, foreground: '#e7e4dc', cursor: '#e2b55c', selectionBackground: '#3a3b3e' },
      })
      const fit = new FitAddon()
      term.loadAddon(fit)
      term.loadAddon(new ClipboardAddon())
      term.open(element)
      try {
        term.loadAddon(new WebglAddon())
      } catch {
        // The DOM renderer is slower but works everywhere.
      }
      fit.fit()
      cleanups.push(() => term.dispose())

      const bridge: Bridge = {
        cols: term.cols,
        rows: term.rows,
        dark: true,
        write: (bytes) => {
          if (!disposed) term.write(bytes)
        },
        load: (key) => {
          try {
            return localStorage.getItem(key)
          } catch {
            return null
          }
        },
        save: (key, value) => {
          try {
            localStorage.setItem(key, value)
          } catch {
            // Private mode or full storage: the session still works, unsaved.
          }
        },
        exit: () => {
          // Go's callbacks throw once the program has exited.
          bridge.input = bridge.resize = bridge.stop = undefined
          if (!disposed) setState('ended')
        },
      }
      window.chesstui = bridge

      try {
        await loadRuntime()
        const go = new window.Go!()
        const instance = await instantiate(go.importObject)
        if (disposed) return
        void go.run(instance)
      } catch {
        if (!disposed) setState('failed')
        return
      }
      setState('running')
      const data = term.onData((text) => bridge.input?.(text))
      const resized = term.onResize(({ cols, rows }) => bridge.resize?.(cols, rows))
      cleanups.push(() => data.dispose(), () => resized.dispose())
      cleanups.push(() => bridge.stop?.())

      const observer = new ResizeObserver(() => {
        term.options.fontSize = fontSize()
        fit.fit()
      })
      observer.observe(element)
      cleanups.push(() => observer.disconnect())
      term.focus()
    }

    setState('loading')
    void start()
    return () => {
      disposed = true
      cleanups.reverse().forEach((cleanup) => {
        try {
          cleanup()
        } catch {
          // Never let a finished program take the page down with it.
        }
      })
    }
  }, [run])

  return (
    <div className="live">
      <div ref={host} className="live-screen" aria-label="chesstui running in your browser" />
      {state !== 'running' ? (
        <div className="live-overlay" role="status">
          {state === 'loading' ? <span>Loading chesstui…</span> : null}
          {state === 'failed' ? <span>The demo could not load. Install chesstui to try it.</span> : null}
          {state === 'ended' ? (
            <>
              <span>Session ended</span>
              <button type="button" className="pill" onClick={() => setRun((n) => n + 1)}>
                Start again
              </button>
            </>
          ) : null}
        </div>
      ) : null}
      <button type="button" className="live-close" onClick={onClose} aria-label="Close the live demo">
        <CloseIcon />
      </button>
    </div>
  )
}

function CloseIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M6 6l12 12M18 6L6 18" />
    </svg>
  )
}
