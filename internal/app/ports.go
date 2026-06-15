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
	// SetKeyHandler はオーバーレイがキー入力を受け取った際のコールバックを設定する。
	SetKeyHandler(func(vk uintptr))
}

// OverlayFactory はモニターに合わせた Overlay を生成する。
type OverlayFactory interface {
	NewOverlay(mon monitor.Info, size spatial.LabelSize) (Overlay, error)
}

// Deps はオーケストレーション本体が必要とする依存(ポート)の集合である。
type Deps struct {
	Input          Input
	Monitors       MonitorLocator
	OverlayFactory OverlayFactory
}
