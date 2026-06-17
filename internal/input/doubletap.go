package input

import "time"

// DoubleTap は特定キー(本アプリでは Shift)の「2回連打」を検出する。
// OS 非依存の純粋ロジックとして実装し、時刻は呼び出し側が渡す(テスト可能性のため)。
//
// 使い方:
//   - 対象キーが新規に押下された(オートリピートでない)ときに Tap(now) を呼ぶ。
//     直前の Tap から window 以内であれば true(=連打成立)を返す。
//   - 対象キー以外のキーが押されたら Reset() を呼ぶ(連打シーケンスを無効化し、
//     大文字入力などの通常操作で誤発火しないようにする)。
type DoubleTap struct {
	window  time.Duration
	last    time.Time
	hasLast bool
}

// NewDoubleTap は連打成立とみなす最大間隔 window を持つ検出器を生成する。
func NewDoubleTap(window time.Duration) *DoubleTap {
	return &DoubleTap{window: window}
}

// Tap は対象キーの新規押下を1回分入力する。直前の押下から window 以内なら
// 連打成立として true を返し、状態をリセットする(3連打目を新たな1打目としない)。
func (d *DoubleTap) Tap(now time.Time) bool {
	if d.hasLast && now.Sub(d.last) <= d.window {
		d.hasLast = false
		return true
	}
	d.last = now
	d.hasLast = true
	return false
}

// Reset は連打シーケンスを破棄する(対象キー以外が押されたときに呼ぶ)。
func (d *DoubleTap) Reset() {
	d.hasLast = false
}
