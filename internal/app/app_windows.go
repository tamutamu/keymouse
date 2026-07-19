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
type winInput struct {
	cursorHideCalls int
	previousCursor  uintptr
}

func (i *winInput) HideCursor() {
	if i.cursorHideCalls != 0 {
		win32.ClearCursor()
		return
	}
	i.cursorHideCalls, i.previousCursor = win32.HideCursor()
}

func (i *winInput) RestoreCursor() {
	if i.cursorHideCalls == 0 {
		return
	}
	win32.RestoreCursor(i.cursorHideCalls, i.previousCursor)
	i.cursorHideCalls = 0
	i.previousCursor = 0
}

func (*winInput) MoveCursor(x, y int) error { return win32.SetCursorPos(x, y) }

func (*winInput) Click(action spatial.ClickAction) error {
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

func (*winInput) ShiftHeld() bool { return win32.IsShiftPressed() }

func (*winInput) ReleaseShift() { win32.ReleaseShift() }

func (*winInput) InjectEscape() { win32.InjectEscape() }

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

func (winOverlayFactory) NewOverlay(area spatial.Rect, size spatial.LabelSize) (Overlay, error) {
	rect := win32.RECT{
		Left: int32(area.X), Top: int32(area.Y), Right: int32(area.X + area.W), Bottom: int32(area.Y + area.H),
	}
	ov, err := overlay.New(rect, size)
	if err != nil {
		return nil, err
	}
	return &overlayAdapter{ov: ov}, nil
}

// overlayAdapter は *overlay.Overlay を Overlay ポートに適合させる。
type overlayAdapter struct{ ov *overlay.Overlay }

func (a *overlayAdapter) ShowLoading() { a.ov.ShowLoading() }
func (a *overlayAdapter) Show(anchors []spatial.Anchor, depth int, action spatial.ClickAction) {
	a.ov.Show(anchors, depth, action)
}
func (a *overlayAdapter) UpdateAnchors(anchors []spatial.Anchor, depth int) {
	a.ov.UpdateAnchors(anchors, depth)
}
func (a *overlayAdapter) Hide()    { a.ov.Hide() }
func (a *overlayAdapter) Destroy() { a.ov.Destroy() }

// winHook は KeyHook ポートを Win32 の低レベルキーボードフックで実装する。
type winHook struct{}

func (winHook) Install(onKey func(vk uint32, down bool) bool) error {
	return win32.SetKeyboardHook(onKey)
}
func (winHook) Remove() { win32.RemoveKeyboardHook() }

// --- 構築と配線 ---

// New は本番用の Win32 アダプタを注入して App を生成する。
func New(cfg settings.Config) (*App, error) {
	deps := Deps{
		Input:          &winInput{},
		Monitors:       winMonitors{},
		OverlayFactory: winOverlayFactory{},
		Hook:           winHook{},
	}
	return newApp(cfg, deps), nil
}

// Run は Win32 資源を初期化し、メッセージループを開始する(終了までブロックする)。
func (a *App) Run() error {
	defer a.deps.Input.RestoreCursor()
	var trayIcon *tray.Tray
	const wmActivationReady = win32.WM_APP + 1
	const wmExecutionReady = win32.WM_APP + 2
	type queuedActivation struct {
		generation uint64
		plan       activationPlan
	}
	activationResults := make(chan queuedActivation, 8)
	executionResults := make(chan executionResult, 8)

	// メッセージウィンドウのウィンドウプロシージャ。トレイ通知とホットキーを捌く。
	handler := func(hwnd uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
		if trayIcon != nil {
			if handled, r := trayIcon.HandleMessage(hwnd, msg, wParam, lParam); handled {
				return true, r
			}
		}
		switch msg {
		case win32.WM_HOTKEY:
			if int(wParam) == input.HotkeyIDContinuous {
				a.toggleContinuousActivation()
			}
			return true, 0
		case wmActivationReady:
			select {
			case result := <-activationResults:
				if result.generation == a.activationGeneration.Load() {
					a.activationPending = false
					a.applyActivation(result.plan)
				}
			default:
			}
			return true, 0
		case wmExecutionReady:
			select {
			case result := <-executionResults:
				a.applyExecution(result)
			default:
			}
			return true, 0
		case win32.WM_DESTROY:
			win32.PostQuitMessage(0)
			return true, 0
		}
		return false, 0
	}

	// トレイとワーカー完了通知を受け取る不可視のメッセージ専用ウィンドウ。
	msgWin, err := win32.CreateMessageWindow("KeyMouseMsgWnd", handler)
	if err != nil {
		return err
	}
	defer msgWin.DestroyWindow()

	// UIA providers can block. Discovery runs on its own COM worker; only HWND
	// creation is marshalled back to the app thread. This keeps the low-level
	// keyboard hook responsive through the entire scan.
	a.requestActivation = func(action spatial.ClickAction, continuous, gridMode bool) {
		a.session.Finish()
		a.cleanup()
		a.gridMode = gridMode
		if !gridMode {
			a.showLoadingOverlay()
		}
		generation := a.activationGeneration.Add(1)
		a.activationPending = true
		go func() {
			plan := a.prepareActivation(action, continuous, gridMode)
			activationResults <- queuedActivation{generation: generation, plan: plan}
			win32.PostMessage(msgWin.HWND, wmActivationReady, 0, 0)
		}()
	}
	a.requestExecution = func(request executionRequest) {
		go func() {
			executionResults <- a.performExecution(request)
			win32.PostMessage(msgWin.HWND, wmExecutionReady, 0, 0)
		}()
	}

	// The only activation trigger: Shift+Space starts continuous mode.
	hotkeyMgr := input.New(msgWin.HWND, []input.Binding{{ID: input.HotkeyIDContinuous, Config: input.HotkeyConfig{VK: vkSpace, Modifiers: input.ModShift | input.ModNoRepeat}}})
	if err := hotkeyMgr.Register(); err != nil {
		return err
	}
	defer hotkeyMgr.Unregister()

	// 低レベルキーボードフックは選択中のフォーカス非依存入力だけに用いる。
	if err := a.deps.Hook.Install(a.onKeyHook); err != nil {
		log.Printf("keyboard hook install failed: %v", err)
	}
	defer a.deps.Hook.Remove()

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
		a.continuous = newCfg.ContinuousModeDefault
		a.peekVK = peekVirtualKey(newCfg.PeekKey)
		if err := tray.SetAutoStart(newCfg.AutoStart); err != nil {
			log.Printf("autostart: %v", err)
		}
	})
}
