//go:build windows

// Package tutorial は初回起動時および設定からの再表示で、操作説明を
// メッセージボックスとして表示する。
package tutorial

import "github.com/tamutamu/keymouse/internal/win32"

const tutorialText = `Welcome to KeyMouse!

How to use:
  1. Press the hotkey (default: Alt+L for left click, Alt+R for right, Alt+D for double).
  2. The screen dims and shows letter labels (A–Y) over a 5×5 grid.
  3. Press a label key to zoom into that region (labels refresh).
  4. Keep pressing label keys to narrow down the target.
  5. Press Shift+<label> at any depth to click immediately.
  6. Press Backspace to go back one level.
  7. Press Esc to cancel without clicking.

The hotkeys and grid size can be changed in Settings (right-click the tray icon).`

// ShowFirstRun はチュートリアルを簡易なメッセージボックスで表示する。
// parent は親ウィンドウハンドル(0 でデスクトップ)。
func ShowFirstRun(parent uintptr) {
	win32.MessageBox(parent, tutorialText, "KeyMouse – Getting Started",
		win32.MB_OK|win32.MB_ICONINFORMATION)
}
