//go:build windows

package settings

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tamutamu/keymouse/internal/win32"
)

// 本ファイルはネイティブ Win32 コントロールによる設定ダイアログを実装する。
// ウィンドウ生成・コントロール生成・値の読み書きはすべて win32 パッケージの
// ラッパー経由で行い、本パッケージから DLL を直接ロードすることはない。

const settingsClassName = "KeyMouseSettings"

// 子コントロールのID
const (
	idEditVKLeft   = 202
	idEditVKRight  = 204
	idEditVKDouble = 206
	idEditCols     = 208
	idEditRows     = 210
	idEditDepth    = 212
	idChkAutoStart = 213
	idChkTutorial  = 214
	idBtnSave      = 215
	idBtnCancel    = 216
)

// settingsWindow は設定ダイアログ 1 つ分の状態を保持する。
type settingsWindow struct {
	window *win32.Window
	cfg    Config
	onSave func(Config)
}

// current は同時に 1 つだけ設定ウィンドウを開くためのシングルトン参照。
var current *settingsWindow

// OpenSettingsWindow は設定ウィンドウを生成して表示する。
// 既に開いている場合は二重に開かない。onSave は保存ボタン押下時に呼ばれる。
func OpenSettingsWindow(parent uintptr, cfg Config, onSave func(Config)) {
	if current != nil && current.window != nil {
		return
	}

	sw := &settingsWindow{cfg: cfg, onSave: onSave}
	w, err := win32.CreateAppWindow(settingsClassName, "KeyMouse Settings",
		100, 100, 400, 420, sw.handleMessage)
	if err != nil {
		return
	}
	sw.window = w
	current = sw
	sw.createControls(w.HWND)
}

// handleMessage は設定ウィンドウのメッセージハンドラ。
// 保存/キャンセルボタンと破棄を処理し、それ以外は既定処理に委ねる。
func (sw *settingsWindow) handleMessage(hwnd uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
	switch msg {
	case win32.WM_COMMAND:
		id, notify := win32.ParseCommand(wParam)
		if notify == win32.BN_CLICKED {
			switch id {
			case idBtnSave:
				sw.saveAndClose()
			case idBtnCancel:
				sw.close()
			}
		}
		return true, 0

	case win32.WM_DESTROY:
		if current == sw {
			current = nil
		}
		sw.window = nil
		return true, 0
	}
	return false, 0
}

// createControls は設定ウィンドウ内に子コントロールを配置する。
func (sw *settingsWindow) createControls(hwnd uintptr) {
	y := 15
	win32.CreateLabel(hwnd, "Hotkeys (virtual key code hex, e.g. 4C for L):", 10, y, 360, 20)
	y += 25
	win32.CreateLabel(hwnd, "Left click (VK):", 10, y, 130, 20)
	win32.CreateEdit(hwnd, idEditVKLeft, fmt.Sprintf("%02X", sw.cfg.HotkeyLeft.VK), 145, y, 60, 22)
	y += 28
	win32.CreateLabel(hwnd, "Right click (VK):", 10, y, 130, 20)
	win32.CreateEdit(hwnd, idEditVKRight, fmt.Sprintf("%02X", sw.cfg.HotkeyRight.VK), 145, y, 60, 22)
	y += 28
	win32.CreateLabel(hwnd, "Double click (VK):", 10, y, 130, 20)
	win32.CreateEdit(hwnd, idEditVKDouble, fmt.Sprintf("%02X", sw.cfg.HotkeyDouble.VK), 145, y, 60, 22)
	y += 38
	win32.CreateLabel(hwnd, "Grid:", 10, y, 130, 20)
	y += 25
	win32.CreateLabel(hwnd, "Columns:", 10, y, 80, 20)
	win32.CreateEdit(hwnd, idEditCols, strconv.Itoa(sw.cfg.Cols), 95, y, 50, 22)
	win32.CreateLabel(hwnd, "Rows:", 175, y, 60, 20)
	win32.CreateEdit(hwnd, idEditRows, strconv.Itoa(sw.cfg.Rows), 240, y, 50, 22)
	y += 28
	win32.CreateLabel(hwnd, "Max depth:", 10, y, 80, 20)
	win32.CreateEdit(hwnd, idEditDepth, strconv.Itoa(sw.cfg.MaxDepth), 95, y, 50, 22)
	y += 38
	win32.CreateCheckbox(hwnd, idChkAutoStart, "Start with Windows", 10, y, 200, 22, sw.cfg.AutoStart)
	y += 28
	win32.CreateCheckbox(hwnd, idChkTutorial, "Show tutorial on next start", 10, y, 220, 22, sw.cfg.ShowTutorialOnce)
	y += 40
	win32.CreateButton(hwnd, idBtnSave, "Save", 220, y, 70, 28, true)
	win32.CreateButton(hwnd, idBtnCancel, "Cancel", 300, y, 70, 28, false)
}

// saveAndClose は入力値を読み取って Config を更新・保存し、ウィンドウを閉じる。
func (sw *settingsWindow) saveAndClose() {
	hwnd := sw.window.HWND

	// 16 進文字列の仮想キーコードを読み取る。解釈できなければ 0 を返す。
	readVK := func(id int) uint32 {
		s := strings.TrimSpace(win32.GetDlgItemText(hwnd, id))
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return 0
		}
		return uint32(v)
	}

	cfg := sw.cfg
	if vk := readVK(idEditVKLeft); vk != 0 {
		cfg.HotkeyLeft.VK = vk
	}
	if vk := readVK(idEditVKRight); vk != 0 {
		cfg.HotkeyRight.VK = vk
	}
	if vk := readVK(idEditVKDouble); vk != 0 {
		cfg.HotkeyDouble.VK = vk
	}
	if v := win32.GetDlgItemInt(hwnd, idEditCols); v > 0 {
		cfg.Cols = v
	}
	if v := win32.GetDlgItemInt(hwnd, idEditRows); v > 0 {
		cfg.Rows = v
	}
	if v := win32.GetDlgItemInt(hwnd, idEditDepth); v > 0 {
		cfg.MaxDepth = v
	}
	cfg.AutoStart = win32.IsDlgButtonChecked(hwnd, idChkAutoStart)
	cfg.ShowTutorialOnce = win32.IsDlgButtonChecked(hwnd, idChkTutorial)

	if err := Save(cfg); err != nil {
		win32.MessageBox(hwnd, "Failed to save settings: "+err.Error(), "Error",
			win32.MB_OK|win32.MB_ICONERROR)
		return
	}

	if sw.onSave != nil {
		sw.onSave(cfg)
	}
	sw.close()
}

// close は設定ウィンドウを破棄する。
func (sw *settingsWindow) close() {
	if sw.window != nil {
		sw.window.DestroyWindow()
	}
}
