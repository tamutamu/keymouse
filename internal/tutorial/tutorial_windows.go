//go:build windows

// Package tutorial は初回起動時および設定からの再表示で、操作説明を
// メッセージボックスとして表示する。
package tutorial

import "github.com/tamutamu/keymouse/internal/win32"

const tutorialText = `Welcome to KeyMouse!

How to use:
  1. Double-tap Shift for a left click (Alt+R/right, Alt+D/double).
  2. Type the three-letter cell label (A/S/D/F/G/T/R/E/W/Q).
  3. The third key clicks the centre of that fixed 40x25 grid cell.
  4. H/J/K/L move by 4 px; Shift+H/J/K/L moves by 16 px.
  5. Backspace removes one letter; Esc cancels without clicking.

The labels and overlay cover the full monitor at the cursor.`

// ShowFirstRun はチュートリアルを簡易なメッセージボックスで表示する。
// parent は親ウィンドウハンドル(0 でデスクトップ)。
func ShowFirstRun(parent uintptr) {
	win32.MessageBox(parent, tutorialText, "KeyMouse – Getting Started",
		win32.MB_OK|win32.MB_ICONINFORMATION)
}
