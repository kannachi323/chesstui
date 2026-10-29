# Design QA

## Sources

- Native Home: `/var/folders/hd/3cy894z92v70m7crvhv8rwmr0000gn/T/TemporaryItems/NSIRD_screencaptureui_vT7JSn/Screenshot 2026-07-17 at 12.34.39 PM.png`
- Native Learn: `/var/folders/hd/3cy894z92v70m7crvhv8rwmr0000gn/T/TemporaryItems/NSIRD_screencaptureui_Dgyjf4/Screenshot 2026-07-17 at 12.35.33 PM.png`
- Previous web implementation: `/var/folders/hd/3cy894z92v70m7crvhv8rwmr0000gn/T/TemporaryItems/NSIRD_screencaptureui_F25ESz/Screenshot 2026-07-17 at 12.34.26 PM.png`

## Verification setup

- Browser viewport: 1024 x 594, matching the native 2048 x 1188 reference at 2x scale.
- Final captures: `/tmp/chesstui-web-home.png` and `/tmp/chesstui-web-learn.png`.
- Browser console: no errors in Home, Watch, Puzzles, or Learn.

## Iteration history

### Iteration 1

The old web screen was a separate imitation: it had a white page frame and web-only title bar, different proportions, no board on the first Learn lesson, non-functional keyboard navigation, Unicode pieces, and mock integrations.

Changes:

- Rebuilt the page as a full-viewport terminal surface using the native theme, wordmark, tab row, centered content, and statusline.
- Matched the native Home and Learn hierarchy and responsive proportions.
- Added real SVG chess artwork through `chessboard-element`.
- Connected public Lichess puzzle, TV, streamer, and broadcast APIs.
- Added browser-safe Lichess OAuth PKCE for account connection.

### Iteration 2

The Learn `g8f6` input initially triggered the global number shortcut before the lesson could consume it.

Changes:

- Prioritized active content input over global number shortcuts.
- Re-tested `g8f6` and confirmed the lesson completes.
- Confirmed `L`, `gt`, number shortcuts, Enter, and UCI move input work.

## Residual differences

- The browser uses SVG piece artwork intentionally, per the request, whereas the native screenshot uses font-dependent Unicode glyphs.
- Terminal and browser font rasterization differ slightly by platform; layout, spacing, hierarchy, and color are matched at the target viewport.
- Authenticated game creation and local Stockfish execution are not exercised in browser QA because they require a user-authorized Lichess session and a browser-specific engine worker.

## Final result

Passed. No P1 or P2 visual or interaction mismatches remain in the tested Home, Watch, Puzzles, and Learn flows.
