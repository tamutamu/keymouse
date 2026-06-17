package app

import (
	"log"
	"time"

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

	// Alt+(h/j/k/l) でグリッド全体を平行移動して微調整する(vim 流)。
	// 左クリック開始ホットキーを Alt+N にしたことで Alt+L が空き、右に使える。
	vkH = 0x48 // h: 左
	vkJ = 0x4A // j: 下
	vkK = 0x4B // k: 上
	vkL = 0x4C // l: 右
)

// panStepPx はグリッド移動 1 回あたりの画面ピクセル移動量。
const panStepPx = 8.0

// shiftDoubleTapWindow は Shift 2連打とみなす最大間隔。
const shiftDoubleTapWindow = 400 * time.Millisecond

// Shift 系の仮想キーコード(左右個別キーを含む)。
const (
	vkShift  = 0x10
	vkLShift = 0xA0
	vkRShift = 0xA1
)

// isShiftVK は vk が Shift 系キーなら true を返す。
func isShiftVK(vk uint32) bool {
	return vk == vkShift || vk == vkLShift || vk == vkRShift
}

// isModifierVK は vk が Shift/Ctrl/Alt のいずれかなら true を返す。
// これらは選択中も飲み込まず通過させる(システムの修飾状態を壊さないため)。
func isModifierVK(vk uint32) bool {
	switch vk {
	case vkShift, vkLShift, vkRShift, // Shift
		0x11, 0xA2, 0xA3, // Ctrl
		0x12, 0xA4, 0xA5: // Alt
		return true
	}
	return false
}

// App は選択セッションのオーケストレーションを担う中核オブジェクトである。
// OS 依存処理はすべて Deps のポート経由で呼び出すため、本体は OS 非依存。
type App struct {
	cfg     settings.Config
	session *session.Session
	deps    Deps

	overlay Overlay

	// shiftTap は待機中の Shift 2連打(オーバーレイ起動操作)を検出する。
	shiftTap *input.DoubleTap
	// shiftDown は Shift の物理押下状態(オートリピートと新規押下の区別に使う)。
	shiftDown bool
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
		cfg:      cfg,
		session:  session.New(spatialCfg, spatial.LabelKeys),
		deps:     deps,
		shiftTap: input.NewDoubleTap(shiftDoubleTapWindow),
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
	a.overlay.Show(a.session.CurrentAnchors(), action)
}

// onKeyHook は常駐キーボードフックからの全キー通知を処理する。
// 戻り値 true はそのキーを飲み込む(背後アプリへ渡さない)ことを意味するが、
// 修飾キーは win32 層が常に通過させるため、実質的に飲み込まれるのは非修飾キーのみ。
//
//   - 待機中(Idle): Shift の2連打でオーバーレイを開く。通常入力は一切飲み込まない。
//   - 選択中(Selecting): 非修飾キーの押下を onKeyDown へ回し、飲み込む。
func (a *App) onKeyHook(vk uint32, down bool) bool {
	// Shift の物理押下状態を更新し、新規押下(オートリピートでない)を判定する。
	freshShiftPress := false
	if isShiftVK(vk) {
		if down {
			freshShiftPress = !a.shiftDown
			a.shiftDown = true
		} else {
			a.shiftDown = false
		}
	}

	if a.session.State() == session.StateSelecting {
		if down && !isModifierVK(vk) {
			a.onKeyDown(uintptr(vk))
		}
		return !isModifierVK(vk) // 非修飾キーは飲み込む(背後へ漏らさない)
	}

	// 待機中: Shift 2連打の検出。誤発火を避けるため、Shift 以外の押下で連打を破棄する。
	switch {
	case freshShiftPress:
		if a.shiftTap.Tap(time.Now()) {
			a.onHotkey(spatial.ClickLeft)
		}
	case down && !isShiftVK(vk):
		a.shiftTap.Reset()
	}
	return false // 待機中は何も飲み込まない
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

	// Alt+(h/j/k/l) はグリッドの平行移動。Alt 押下中のキーはここで消費し、
	// ラベル選択には回さない(移動キー以外の Alt+キーは無視する)。
	if a.deps.Input.AltHeld() {
		a.panGrid(vk)
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

// panGrid は Alt+(h/j/k/;) に応じて現在のグリッドを平行移動し、再描画する。
// 移動キー以外では何もしない。
func (a *App) panGrid(vk uintptr) {
	var dx, dy float64
	switch vk {
	case vkH:
		dx = -panStepPx
	case vkJ:
		dy = panStepPx
	case vkK:
		dy = -panStepPx
	case vkL:
		dx = panStepPx
	default:
		return // 移動キーでなければ無視
	}
	if a.session.PanCurrentStage(dx, dy) {
		if anchors := a.session.CurrentAnchors(); anchors != nil {
			a.overlay.UpdateAnchors(anchors)
		}
	}
}

// executeClick はオーバーレイを隠し、カーソルを移動してクリックを送信する。
func (a *App) executeClick(anchor spatial.Anchor) {
	if a.overlay != nil {
		a.overlay.Hide()
	}

	// Shift+ラベルで即クリックした場合、物理的に押された Shift がクリックに
	// 漏れて Shift+クリックになるのを防ぐため、クリック前に Shift を解除する。
	a.deps.Input.ReleaseShift()

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
// キーボードフックは Shift 2連打の検出のため常駐させ続けるので、ここでは解除しない。
func (a *App) cleanup() {
	if a.overlay != nil {
		a.overlay.Destroy()
		a.overlay = nil
	}
}
