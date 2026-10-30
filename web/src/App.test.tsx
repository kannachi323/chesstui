import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App, { HeroTerminal, INSTALL_COMMAND, InstallCommand, REPO_URL } from './App'

function visibleFrame() {
  return screen.getAllByTestId('hero-frame').findIndex((img) => img.classList.contains('is-visible'))
}

describe('landing page', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    globalThis.fetch = vi.fn().mockRejectedValue(new Error('offline'))
    window.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof window.matchMedia
  })
  afterEach(() => vi.useRealTimers())

  it('leads with the headline, install command and GitHub link', () => {
    render(<App />)
    expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('Chess in the terminal.')
    expect(document.querySelector('.install code')?.textContent).toBe(INSTALL_COMMAND)
    const github = screen.getAllByRole('link').filter((link) => link.getAttribute('href') === REPO_URL)
    expect(github.length).toBeGreaterThan(0)
    expect(screen.getAllByRole('img').length).toBeGreaterThanOrEqual(4)
  })

  it('copies the install command', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    render(<InstallCommand />)
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Copy install command' }))
    })
    expect(writeText).toHaveBeenCalledWith(INSTALL_COMMAND)
    expect(screen.getByRole('button', { name: 'Copied' })).toBeTruthy()
    act(() => vi.advanceTimersByTime(2000))
    expect(screen.getByRole('button', { name: 'Copy install command' })).toBeTruthy()
  })

  it('steps through the hero frames', () => {
    render(<HeroTerminal interval={100} />)
    expect(visibleFrame()).toBe(0)
    act(() => vi.advanceTimersByTime(100))
    expect(visibleFrame()).toBe(1)
    act(() => vi.advanceTimersByTime(300))
    expect(visibleFrame()).toBe(4)
  })

  it('offers the live demo only when the WebAssembly build is there', async () => {
    const wasm = new Response(null, { headers: { 'content-type': 'application/wasm' } })
    const page = new Response(null, { headers: { 'content-type': 'text/html' } })
    globalThis.fetch = vi.fn().mockResolvedValueOnce(page).mockResolvedValueOnce(wasm)
    const { unmount } = render(<HeroTerminal />)
    await act(async () => {})
    expect(screen.queryByRole('button', { name: 'Try it in your browser' })).toBeNull()
    unmount()
    render(<HeroTerminal />)
    await act(async () => {})
    expect(screen.getByRole('button', { name: 'Try it in your browser' })).toBeTruthy()
  })

  it('holds the final frame when motion is reduced', () => {
    window.matchMedia = vi.fn().mockReturnValue({ matches: true }) as unknown as typeof window.matchMedia
    render(<HeroTerminal interval={100} />)
    const last = screen.getAllByTestId('hero-frame').length - 1
    expect(visibleFrame()).toBe(last)
    act(() => vi.advanceTimersByTime(1000))
    expect(visibleFrame()).toBe(last)
  })
})
