//go:build windows

package settings

import (
	"github.com/tamutamu/keymouse/internal/win32"
)

// 本ファイルはネイティブ Win32 コントロールによる設定ダイアログを実装する。
// ウィンドウ生成・コントロール生成・値の読み書きはすべて win32 パッケージの
// ラッパー経由で行い、本パッケージから DLL を直接ロードすることはない。
//
// 設定項目は「PC起動時に起動するか(AutoStart)」のみ。グリッドやラベルサイズ等の
// 動作パラメータは config.json での手編集に委ね、ダイアログには公開しない。

const settingsClassName = "KeyMouseSettings"

// 子コントロールのID
const (
	idChkAutoStart = 213
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
		100, 100, 300, 150, sw.handleMessage)
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
// 公開する設定は「PC起動時に起動する」チェックボックスのみ。
func (sw *settingsWindow) createControls(hwnd uintptr) {
	win32.CreateCheckbox(hwnd, idChkAutoStart, "Start with Windows", 15, 20, 220, 22, sw.cfg.AutoStart)
	win32.CreateButton(hwnd, idBtnSave, "Save", 110, 65, 80, 28, true)
	win32.CreateButton(hwnd, idBtnCancel, "Cancel", 195, 65, 80, 28, false)
}

// saveAndClose は入力値を読み取って Config を更新・保存し、ウィンドウを閉じる。
// 編集対象は AutoStart のみで、それ以外のフィールドは元の値をそのまま書き戻す。
func (sw *settingsWindow) saveAndClose() {
	hwnd := sw.window.HWND

	cfg := sw.cfg
	cfg.AutoStart = win32.IsDlgButtonChecked(hwnd, idChkAutoStart)

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
