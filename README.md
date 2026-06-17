# keymouse

README: English | [日本語](README_ja.md)

keymouse is a Windows tray app for clicking any point on the screen using only
the keyboard. It opens a lightweight overlay on the monitor under the cursor,
narrows the target area through labeled grid selections, and sends a left,
right, or double click at the selected point.

## Features

- Keyboard-only pointer targeting for Windows
- Left-click targeting with a quick double-tap of `Shift`
- Global hotkeys for right click (`Alt+R`) and double click (`Alt+D`)
- Hierarchical `5 x 5` label grid using single-letter labels (`A` through `Y`)
- `Shift` + label to click immediately at the current grid level
- `Backspace` to go back one level, `Esc` to cancel
- `Alt+H/J/K/L` to nudge the current grid left/down/up/right
- Per-monitor operation based on the monitor currently under the cursor
- Tray icon with Settings and Exit actions
- Optional Windows startup registration
- Local-only behavior; no telemetry or network communication

## Requirements

- Windows 11
- Windows 10 may work, but Windows 11 is the primary target
- Go 1.22 or later to build from source

## Build

From PowerShell:

```powershell
.\build.ps1
```

Or with `go build` directly:

```powershell
go build -ldflags "-H windowsgui" -o keymouse.exe ./cmd/app
```

The `windowsgui` subsystem prevents a console window from appearing when the
tray app starts.

## Usage

1. Start `keymouse.exe`.
2. Use the tray icon to open Settings or Exit.
3. Double-tap `Shift` to start a left-click selection.
4. Press a label key (`A` through `Y`) to zoom into that area.
5. Press `Shift` + label to click immediately, or continue selecting labels
   until the final level clicks automatically.
6. Use `Alt+R` for a right-click selection, and `Alt+D` for a double-click
   selection.

During selection:

- `Backspace`: return to the previous level
- `Esc`: cancel
- `Alt+H`: move the current grid left
- `Alt+J`: move the current grid down
- `Alt+K`: move the current grid up
- `Alt+L`: move the current grid right

## Configuration

Settings are stored at:

```text
%APPDATA%\keymouse\config.json
```

The Settings window currently exposes Windows startup registration. Other
behavior, such as hotkeys, grid size, depth, and label size, can be adjusted by
editing `config.json`.

Default values include:

- Grid: `5 x 5`
- Maximum depth: `3`
- Right click hotkey: `Alt+R`
- Double click hotkey: `Alt+D`
- Startup registration: disabled

Hotkey and grid changes should be made while the app is not running, or applied
after restarting the app.

## Development

Run tests:

```powershell
go test ./...
```

Build:

```powershell
.\build.ps1
```

The main entry point is `cmd/app`, with implementation packages under
`internal/`.

## License

MIT
