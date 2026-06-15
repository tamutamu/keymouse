package app

import (
	"log"

	"github.com/tamutamu/keymouse/internal/input"
	"github.com/tamutamu/keymouse/internal/monitor"
	"github.com/tamutamu/keymouse/internal/session"
	"github.com/tamutamu/keymouse/internal/settings"
	"github.com/tamutamu/keymouse/internal/spatial"
)

// 選択中に特別扱いするキーの仮想キーコード。
const (
	vkEscape = 0x1B // Esc: キャンセル
	vkBack   = 0x08 // Backspace: 1 段階戻る
)

// App は選択セッションのオーケストレーションを担う中核オブジェクトである。
// OS 依存処理はすべて Deps のポート経由で呼び出すため、本体は OS 非依存。
type App struct {
	cfg     settings.Config
	session *session.Session
	deps    Deps

	overlay Overlay
}

// newApp は設定と依存ポートから App を構築する(OS 非依存の内部コンストラクタ)。
func newApp(cfg settings.Config, deps Deps) *App {
	spatialCfg := spatial.Config{
		Cols:       cfg.Cols,
		Rows:       cfg.Rows,
		MinLabelPx: cfg.MinLabelPx,
		MaxDepth:   cfg.MaxDepth,
	}
	return &App{
		cfg:     cfg,
		session: session.New(spatialCfg, spatial.LabelKeys),
		deps:    deps,
	}
}

// rectFromMonitor はモニター情報を spatial.Rect(物理ピクセル座標)へ変換する。
func rectFromMonitor(m monitor.Info) spatial.Rect {
	return spatial.Rect{
		X: float64(m.Left),
		Y: float64(m.Top),
		W: float64(m.Width()),
		H: float64(m.Height()),
	}
}

// onHotkey は指定クリック種別で新しい選択セッションを開始する。
func (a *App) onHotkey(action spatial.ClickAction) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in onHotkey: %v", r)
			a.cancelSession()
		}
	}()

	mon, err := a.deps.Monitors.MonitorAtCursor()
	if err != nil {
		log.Printf("monitor at cursor: %v", err)
		return
	}

	// 多重起動防止: 進行中ならここで弾かれる。Start が初期段(グリッド予定表)を構築する。
	if !a.session.Start(action, rectFromMonitor(mon)) {
		return
	}

	labelSize := a.cfg.LabelSize
	if labelSize == 0 {
		labelSize = spatial.LabelNormal
	}
	if a.overlay != nil {
		a.overlay.Destroy()
	}
	ov, err := a.deps.OverlayFactory.NewOverlay(mon, labelSize)
	if err != nil {
		log.Printf("overlay: %v", err)
		a.cancelSession()
		return
	}
	a.overlay = ov
	a.overlay.SetKeyHandler(a.onKeyDown)
	a.overlay.Show(a.session.CurrentAnchors(), action)
}

// onKeyDown はオーバーレイから渡された WM_KEYDOWN を処理する。
func (a *App) onKeyDown(vk uintptr) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in onKeyDown: %v", r)
			a.cancelSession()
		}
	}()

	if a.session.State() != session.StateSelecting {
		return
	}

	switch vk {
	case vkEscape:
		// Esc: カーソル位置・クリック状態を変更せずにキャンセルする。
		a.cancelSession()

	case vkBack:
		// Backspace: 1 段階前の検索領域へ戻る。
		if a.session.Backspace() {
			if anchors := a.session.CurrentAnchors(); anchors != nil {
				a.overlay.UpdateAnchors(anchors)
			}
		}

	default:
		if !input.IsLabelKey(vk) {
			return // 無効キーは無視する。
		}
		key := spatial.Key(vk)

		if a.deps.Input.ShiftHeld() {
			// Shift+ラベル: 現在の段で即クリックする。
			if anchor, ok := a.session.SelectKeyWithClick(key); ok {
				a.executeClick(anchor)
			}
			return
		}

		anchor, shouldClick, advanced := a.session.SelectKey(key)
		switch {
		case shouldClick:
			// 最終段に到達したため自動クリックする。
			a.executeClick(anchor)
		case advanced:
			if anchors := a.session.CurrentAnchors(); anchors != nil {
				a.overlay.UpdateAnchors(anchors)
			}
		}
	}
}

// executeClick はオーバーレイを隠し、カーソルを移動してクリックを送信する。
func (a *App) executeClick(anchor spatial.Anchor) {
	if a.overlay != nil {
		a.overlay.Hide()
	}

	x, y := spatial.SourcePointPhysical(anchor)
	if err := a.deps.Input.MoveCursor(x, y); err != nil {
		log.Printf("move cursor: %v", err)
		a.session.FinishExecuting()
		a.cleanup()
		return
	}

	if err := a.deps.Input.Click(a.session.ClickAction()); err != nil {
		log.Printf("click failed: %v", err)
	}

	a.session.FinishExecuting()
	a.cleanup()
}

// cancelSession はセッションをキャンセルし、オーバーレイを破棄する。
func (a *App) cancelSession() {
	a.session.Cancel()
	a.cleanup()
}

// cleanup はオーバーレイを破棄する(セッション終了時の後始末)。
func (a *App) cleanup() {
	if a.overlay != nil {
		a.overlay.Destroy()
		a.overlay = nil
	}
}
