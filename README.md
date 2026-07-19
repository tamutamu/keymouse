# keymouse

README: English | [日本語](README_ja.md)

KeyMouse is a lightweight Windows tray app for discovering and clicking UI
targets without leaving the keyboard. It combines Windows UI Automation element
hints with the existing fixed coordinate grid. UI Automation is used only for
discovery; selection always performs a real mouse click.

No screen capture, OpenCV, telemetry, or network communication is used.

## Modes

Press `Shift+Space` to start continuous Element Mode. KeyMouse scans the active
window, shows `loading...` while discovery is running, and places short lowercase
labels on visible actionable elements. Labels use the shortest available length
and grow to two or three characters only when the candidate count requires it.

Automatic UIA-to-grid fallback is not used. While labels are visible, press
`Shift+G` to switch to Grid Mode and press it again to rescan and return to
Element Mode. Grid Mode divides the cursor monitor into a fixed `40 × 25` grid
with three-character labels made from `a s d f g t r e w q`.

## Controls

- lowercase label keys: filter candidates and click the completed target
- `Backspace`: remove the last label character
- `H/J/K/L`: move labels left/down/up/right by 4 px
- `Shift+H/J/K/L`: move labels by 16 px
- `Shift+R`: rescan and regenerate labels in the current mode
- hold `Space`: temporarily hide the overlay (peek)
- `Shift+G`: toggle Element Mode and Grid Mode
- `Shift+Space` or `Esc`: end continuous selection

The overlay is hidden before KeyMouse moves the pointer and sends the physical
click. Continuous mode then refreshes the visible targets for the next action.

## Inspector

```powershell
keymouse.exe inspect
```

Inspector writes the foreground window's UI Automation tree as JSON, including
names, control types, Automation IDs, classes, bounds, state, supported actions,
and depth. It is intended for diagnosing provider differences and missing hints.

## Build

```powershell
.\build.ps1
go test ./...
```

Or build directly:

```powershell
go build -ldflags "-H windowsgui" -o keymouse.exe ./cmd/app
```

## Requirements

- Windows 10 or 11
- Go 1.22+ for source builds

## License

MIT
