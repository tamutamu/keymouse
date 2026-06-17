// Package app はトレイ→ホットキー→セッション→クリックという全体の配線と、
// 選択操作のオーケストレーションを担う。
//
// OS 依存処理(Win32)はすべて本ファイルのインターフェース(ポート)の背後に隠蔽し、
// オーケストレーション本体(app.go)はこれらのポートと純粋ドメイン
// (session / spatial / capture / monitor)のみに依存する。これにより
// オーケストレーションロジックをモックでテスト可能にしている。
package app

import (
	"github.com/tamutamu/keymouse/internal/monitor"
	"github.com/tamutamu/keymouse/internal/spatial"
)

// Input はマウス入力の送信と修飾キー状態の取得を抽象化する。
type Input interface {
	// MoveCursor はカーソルを物理スクリーン座標へ移動する。
	MoveCursor(x, y int) error
	// Click は指定種別のクリックを送信する。
	Click(action spatial.ClickAction) error
	// ShiftHeld はいずれかの Shift キーが押されていれば true を返す。
	ShiftHeld() bool
	// AltHeld は Alt キーが押されていれば true を返す。Alt+h/j/k/; のグリッド移動判定に使う。
	AltHeld() bool
	// ReleaseShift は Shift キーの押下状態を解除する。Shift+ラベルで即クリック
	// する際、押されたままの Shift がクリックに漏れる(Shift+クリックになる)のを防ぐ。
	ReleaseShift()
}

// MonitorLocator はカーソル位置にあるモニターの解決を抽象化する。
type MonitorLocator interface {
	MonitorAtCursor() (monitor.Info, error)
}

// Overlay は選択用オーバーレイの表示・更新・破棄を抽象化する。
type Overlay interface {
	Show(anchors []spatial.Anchor, action spatial.ClickAction)
	UpdateAnchors(anchors []spatial.Anchor)
	Hide()
	Destroy()
}

// OverlayFactory はモニターに合わせた Overlay を生成する。
type OverlayFactory interface {
	NewOverlay(mon monitor.Info, size spatial.LabelSize) (Overlay, error)
}

// KeyHook は選択中のグローバルなキー入力取得を抽象化する。オーバーレイはフォーカスを
// 奪わない(背後のメニュー等を維持する)ため、キーはフォーカス非依存のフックで受け取る。
type KeyHook interface {
	// Install はフックを設置する。onKey には押下/離上(down)と仮想キーが渡され、
	// true を返したキーは他アプリへ届かない(飲み込まれる)。
	Install(onKey func(vk uint32, down bool) bool) error
	// Remove はフックを解除する。
	Remove()
}

// Deps はオーケストレーション本体が必要とする依存(ポート)の集合である。
type Deps struct {
	Input          Input
	Monitors       MonitorLocator
	OverlayFactory OverlayFactory
	Hook           KeyHook
}
