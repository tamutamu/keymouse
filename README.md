# keymouse

README: English | [日本語](README_ja.md)

Control Windows with short keyboard labels.

[![KeyMouse demo](assets/demo-preview.gif)](assets/demo.mp4)

The demo shows Element Mode finding links and buttons, label filtering, physical
clicking, and switching to the coordinate grid. Click it to watch the full video.

## How it feels

1. Press `Shift+Space` to show labels on visible UI elements.
2. Type a lowercase label to click that target.
3. Press `Shift+G` when you want the coordinate grid instead.
4. Press `Shift+Space` again to hide all labels.

KeyMouse stays active after a click, refreshes the screen, and waits for the next
label. This makes it practical to complete several operations without reaching
for the mouse.

## What it can do

- Label buttons, links, fields, tabs, menus, and other UI Automation elements
- Click the selected target with a real mouse click
- Reach UIA-inaccessible areas with a fixed `40 × 25` grid
- Keep labels short: one character when possible, then two or three
- Inspect a window's UI Automation tree as JSON with `keymouse.exe inspect`

## Controls

| Key | Action |
|---|---|
| `Shift+Space` | Show Element Mode / hide labels |
| `Shift+G` | Switch between Element Mode and Grid Mode |
| lowercase label | Filter and click a target |
| `Backspace` | Remove the last label character |
| `H/J/K/L` | Move labels left/down/up/right |
| `Shift+H/J/K/L` | Move labels farther |
| `Shift+R` | Refresh labels |
| hold `Space` | Temporarily hide labels |
| `Esc` | Exit |

## Build

```powershell
.\build.ps1
go test ./...
```

Requires Windows 10 or 11. Source builds require Go 1.22+.

## License

MIT
