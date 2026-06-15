//go:build windows

package app

import (
	"log"

	"github.com/tamutamu/keymouse/internal/input"
	"github.com/tamutamu/keymouse/internal/monitor"
	"github.com/tamutamu/keymouse/internal/overlay"
	"github.com/tamutamu/keymouse/internal/settings"
	"github.com/tamutamu/keymouse/internal/spatial"
	"github.com/tamutamu/keymouse/internal/tray"
	"github.com/tamutamu/keymouse/internal/tutorial"
	"github.com/tamutamu/keymouse/internal/win32"
)

// 本ファイルは Windows 専用の具象アダプタ(ポートの実装)と、実際の Win32 資源
// (メッセージウィンドウ・ホットキー・トレイ)の配線を担う。
// オーケストレーション本体は OS 非依存の app.go 側にある。

// --- ポートの Win32 実装(アダプタ) ---

// winInput は Input ポートを Win32 のマウス入力/キー状態で実装する。
type winInput struct{}

func (winInput) MoveCursor(x, y int) error { return win32.SetCursorPos(x, y) }

func (winInput) Click(action spatial.ClickAction) error {
	switch action {
	case spatial.ClickLeft:
		return win32.ClickLeft()
	case spatial.ClickRight:
		return win32.ClickRight()
	case spatial.ClickDouble:
		return win32.ClickDouble()
	}
	return nil
}

func (winInput) ShiftHeld() bool { return win32.IsShiftPressed() }

func (winInput) AltHeld() bool { return win32.IsAltPressed() }

func (winInput) ReleaseShift() { win32.ReleaseShift() }

// winMonitors は MonitorLocator ポートを Win32 で実装する。
type winMonitors struct{}

func (winMonitors) MonitorAtCursor() (monitor.Info, error) {
	cx, cy, err := win32.GetCursorPos()
	if err != nil {
		return monitor.Info{}, err
	}
	m, err := win32.MonitorFromPoint(cx, cy)
	if err != nil {
		return monitor.Info{}, err
	}
	return monitor.Info{
		Left:   int(m.Rect.Left),
		Top:    int(m.Rect.Top),
		Right:  int(m.Rect.Right),
		Bottom: int(m.Rect.Bottom),
	}, nil
}

// winOverlayFactory は OverlayFactory ポートを overlay パッケージで実装する。
type winOverlayFactory struct{}

func (winOverlayFactory) NewOverlay(mon monitor.Info, size spatial.LabelSize) (Overlay, error) {
	rect := win32.RECT{
		Left:   int32(mon.Left),
		Top:    int32(mon.Top),
		Right:  int32(mon.Right),
		Bottom: int32(mon.Bottom),
	}
	ov, err := overlay.New(rect, size)
	if err != nil {
		return nil, err
	}
	return &overlayAdapter{ov: ov}, nil
}

// overlayAdapter は *overlay.Overlay を Overlay ポートに適合させる。
type overlayAdapter struct{ ov *overlay.Overlay }

func (a *overlayAdapter) Show(anchors []spatial.Anchor, action spatial.ClickAction) {
	a.ov.Show(anchors, action)
}
func (a *overlayAdapter) UpdateAnchors(anchors []spatial.Anchor) { a.ov.UpdateAnchors(anchors) }
func (a *overlayAdapter) Hide()                                  { a.ov.Hide() }
func (a *overlayAdapter) Destroy()                               { a.ov.Destroy() }
func (a *overlayAdapter) SetKeyHandler(fn func(vk uintptr))      { a.ov.OnKeyDown = fn }

// --- 構築と配線 ---

// New は本番用の Win32 アダプタを注入して App を生成する。
func New(cfg settings.Config) (*App, error) {
	deps := Deps{
		Input:          winInput{},
		Monitors:       winMonitors{},
		OverlayFactory: winOverlayFactory{},
	}
	return newApp(cfg, deps), nil
}

// Run は Win32 資源を初期化し、メッセージループを開始する(終了までブロックする)。
func (a *App) Run() error {
	var trayIcon *tray.Tray

	// メッセージウィンドウのウィンドウプロシージャ。トレイ通知とホットキーを捌く。
	handler := func(hwnd uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
		if trayIcon != nil {
			if handled, r := trayIcon.HandleMessage(hwnd, msg, wParam, lParam); handled {
				return true, r
			}
		}
		switch msg {
		case win32.WM_HOTKEY:
			if action, ok := input.ActionForHotkeyID(wParam); ok {
				a.onHotkey(action)
			}
			return true, 0
		case win32.WM_DESTROY:
			win32.PostQuitMessage(0)
			return true, 0
		}
		return false, 0
	}

	// WM_HOTKEY やトレイのコールバックを受け取る不可視のメッセージ専用ウィンドウ。
	msgWin, err := win32.CreateMessageWindow("KeyMouseMsgWnd", handler)
	if err != nil {
		return err
	}
	defer msgWin.DestroyWindow()

	// ホットキー登録。失敗時は設定画面を開いて変更を促す。
	hotkeyMgr := input.New(msgWin.HWND, hotkeyConfigFrom(a.cfg))
	if err := hotkeyMgr.Register(); err != nil {
		log.Printf("hotkey registration failed: %v — edit config.json or settings to change hotkeys", err)
		a.openSettings(msgWin.HWND)
	}
	defer hotkeyMgr.Unregister()

	// タスクトレイ常駐。
	trayIcon = tray.New(msgWin.HWND)
	trayIcon.OnSettings = func() { a.openSettings(msgWin.HWND) }
	trayIcon.OnExit = func() { win32.PostQuitMessage(0) }
	if err := trayIcon.Add(); err != nil {
		log.Printf("tray icon: %v", err)
	}
	defer trayIcon.Remove()

	// 初回起動時のチュートリアル表示。
	if a.cfg.ShowTutorialOnce {
		tutorial.ShowFirstRun(msgWin.HWND)
		a.cfg.ShowTutorialOnce = false
		_ = settings.Save(a.cfg)
	}

	win32.RunMessageLoop()
	return nil
}

// openSettings は設定ウィンドウを開き、保存時に設定の反映と自動起動の切替を行う。
func (a *App) openSettings(parent uintptr) {
	settings.OpenSettingsWindow(parent, a.cfg, func(newCfg settings.Config) {
		log.Printf("settings saved — restart required for hotkey/grid changes to take effect")
		a.cfg = newCfg
		if err := tray.SetAutoStart(newCfg.AutoStart); err != nil {
			log.Printf("autostart: %v", err)
		}
	})
}

// hotkeyConfigFrom は settings.Config からホットキー構成マップを作る。
func hotkeyConfigFrom(cfg settings.Config) map[spatial.ClickAction]input.HotkeyConfig {
	return map[spatial.ClickAction]input.HotkeyConfig{
		spatial.ClickLeft:   cfg.HotkeyLeft,
		spatial.ClickRight:  cfg.HotkeyRight,
		spatial.ClickDouble: cfg.HotkeyDouble,
	}
}
