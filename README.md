# keymouse

README: English | [日本語](README_ja.md)

KeyMouse is a small Windows tray app for clicking anywhere while keeping your
hands on the keyboard. It uses a fixed, learnable `40 × 25` grid: every cell
has a three-letter label made from `A S D F G T R E W Q` (`AAA`, `AAS`, `AAD`, …).

No screen capture, OpenCV, element detection, telemetry, or network access is
used.

## Use

1. Double-tap `Shift` to start a left-click selection. Use `Alt+R` for right
   click or `Alt+D` for double click.
2. The monitor containing the cursor is covered by the 1,000-cell grid.
3. Type the three letters shown in the target cell. The third key immediately
   clicks its centre.

While the grid is visible:

- `H/J/K/L`: move the whole grid left/down/up/right by 4 px and clear partial input
- `Shift+H/J/K/L`: move by 16 px and clear partial input
- `Backspace`: remove the last typed label letter
- `Esc`: cancel

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
