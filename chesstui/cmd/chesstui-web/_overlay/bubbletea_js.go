//go:build js

package tea

// Bubble Tea has no js/wasm port. The browser build feeds input and resize
// events in itself, so these are the same no-ops as on Windows without a
// console.

func (p *Program) listenForResize(done chan struct{}) { close(done) }

func (p *Program) initInput() error { return nil }

const suspendSupported = false

func suspendProcess() {}
