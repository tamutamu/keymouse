package input

import (
	"testing"
	"time"
)

func TestDoubleTap(t *testing.T) {
	base := time.Unix(1000, 0)
	d := NewDoubleTap(400 * time.Millisecond)

	// 1打目: 成立しない。
	if d.Tap(base) {
		t.Fatal("single tap should not trigger")
	}
	// window 内の2打目: 成立する。
	if !d.Tap(base.Add(300 * time.Millisecond)) {
		t.Fatal("second tap within window should trigger")
	}
	// 成立後はリセットされ、次の1打目は成立しない(3連打目を誤検出しない)。
	if d.Tap(base.Add(350 * time.Millisecond)) {
		t.Fatal("tap right after a trigger should be treated as a fresh first tap")
	}
}

func TestDoubleTapTooSlow(t *testing.T) {
	base := time.Unix(2000, 0)
	d := NewDoubleTap(400 * time.Millisecond)

	if d.Tap(base) {
		t.Fatal("first tap should not trigger")
	}
	// window を超えた2打目: 成立しない(これが新たな1打目になる)。
	if d.Tap(base.Add(500 * time.Millisecond)) {
		t.Fatal("tap after window should not trigger")
	}
	// 直前を1打目として window 内に押せば成立。
	if !d.Tap(base.Add(700 * time.Millisecond)) {
		t.Fatal("subsequent in-window tap should trigger")
	}
}

func TestDoubleTapResetByOtherKey(t *testing.T) {
	base := time.Unix(3000, 0)
	d := NewDoubleTap(400 * time.Millisecond)

	d.Tap(base) // 1打目
	d.Reset()   // 間に別キー
	if d.Tap(base.Add(100 * time.Millisecond)) {
		t.Fatal("tap after Reset must not trigger (sequence invalidated)")
	}
}
