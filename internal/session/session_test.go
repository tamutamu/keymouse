package session

import (
	"testing"

	"github.com/tamutamu/keymouse/internal/spatial"
)

// fhd は 1920×1080 のモニター矩形。MinLabelPx=20 のとき予定表は
// [5×5, 5×5, 3×2] の3段になる。
var fhd = spatial.Rect{X: 0, Y: 0, W: 1920, H: 1080}

// newSession は指定した安全上限 maxDepth を持つセッションを生成する。
func newSession(maxDepth int) *Session {
	cfg := spatial.Config{Cols: 5, Rows: 5, MinLabelPx: 20, MaxDepth: maxDepth}
	return New(cfg, spatial.LabelKeys)
}

// TestSessionIdle は新規セッションが Idle 状態で始まることを検証する。
func TestSessionIdle(t *testing.T) {
	s := newSession(3)
	if s.State() != StateIdle {
		t.Fatalf("new session should be Idle, got %s", s.State())
	}
}

// TestSessionStart は Start で直接 Selecting 状態へ遷移し、初期段(5×5=25個)の
// アンカーが生成されることを検証する。
func TestSessionStart(t *testing.T) {
	s := newSession(3)
	if !s.Start(spatial.ClickLeft, fhd) {
		t.Fatal("Start should succeed from Idle")
	}
	if s.State() != StateSelecting {
		t.Fatalf("after Start: expected Selecting, got %s", s.State())
	}
	if len(s.CurrentAnchors()) != 25 {
		t.Fatalf("expected 25 anchors, got %d", len(s.CurrentAnchors()))
	}
	if s.CurrentDepth() != 1 {
		t.Fatalf("expected depth 1, got %d", s.CurrentDepth())
	}
}

// TestSessionDoubleStartIgnored はセッション進行中の二重 Start が無視され、
// 元のクリック種別が保持されることを検証する。
func TestSessionDoubleStartIgnored(t *testing.T) {
	s := newSession(3)
	s.Start(spatial.ClickLeft, fhd)

	if s.Start(spatial.ClickRight, fhd) {
		t.Fatal("second Start should be ignored (return false)")
	}
	if s.ClickAction() != spatial.ClickLeft {
		t.Fatal("action should remain ClickLeft")
	}
}

// TestSessionSelectKeyAdvances は予定表に余裕がある段でキー選択するとクリックせず
// 次の段階（深さ2）へ進むことを検証する。
func TestSessionSelectKeyAdvances(t *testing.T) {
	s := newSession(3)
	s.Start(spatial.ClickLeft, fhd)

	_, shouldClick, advanced := s.SelectKey(spatial.LabelKeys[0])
	if shouldClick {
		t.Fatal("should not click at depth 1 of a 3-stage schedule")
	}
	if !advanced {
		t.Fatal("should advance to next stage")
	}
	if s.CurrentDepth() != 2 {
		t.Fatalf("expected depth 2, got %d", s.CurrentDepth())
	}
}

// TestSessionBackspace は Backspace で前段階に戻れること、および最小深さ（1）では
// それ以上戻れず false を返すことを検証する。
func TestSessionBackspace(t *testing.T) {
	s := newSession(3)
	s.Start(spatial.ClickLeft, fhd)
	s.SelectKey(spatial.LabelKeys[0]) // 深さ2

	if !s.Backspace() {
		t.Fatal("Backspace from depth 2 should succeed")
	}
	if s.CurrentDepth() != 1 {
		t.Fatalf("after Backspace: expected depth 1, got %d", s.CurrentDepth())
	}
	if s.Backspace() {
		t.Fatal("Backspace from depth 1 (minimum) should return false")
	}
}

// TestSessionCancel は Cancel で Idle 状態に戻り、アンカーがクリアされることを検証する。
func TestSessionCancel(t *testing.T) {
	s := newSession(3)
	s.Start(spatial.ClickLeft, fhd)

	s.Cancel()
	if s.State() != StateIdle {
		t.Fatalf("after Cancel: expected Idle, got %s", s.State())
	}
	if s.CurrentAnchors() != nil {
		t.Fatal("anchors should be cleared after Cancel")
	}
}

// TestSessionSelectKeyWithClick は SelectKeyWithClick が即座にクリックを実行して
// Executing 状態へ遷移し、FinishExecuting で Idle に戻ることを検証する。
func TestSessionSelectKeyWithClick(t *testing.T) {
	s := newSession(3)
	s.Start(spatial.ClickLeft, fhd)

	firstKey := spatial.LabelKeys[3]
	anchor, ok := s.SelectKeyWithClick(firstKey)
	if !ok {
		t.Fatal("SelectKeyWithClick should succeed for a valid key")
	}
	if anchor.Label != firstKey {
		t.Fatalf("expected label %v, got %v", firstKey, anchor.Label)
	}
	if s.State() != StateExecuting {
		t.Fatalf("expected Executing state, got %s", s.State())
	}

	s.FinishExecuting()
	if s.State() != StateIdle {
		t.Fatalf("after FinishExecuting: expected Idle, got %s", s.State())
	}
}

// TestSessionAtLastStageAutoClick は予定表の最終段に達したキー選択で、追加キーなしに
// 自動クリックが発生することを検証する(安全上限 MaxDepth=2 で予定表を2段に絞る)。
func TestSessionAtLastStageAutoClick(t *testing.T) {
	s := newSession(2) // 予定表は [5×5, 5×5] の2段に制限される
	s.Start(spatial.ClickLeft, fhd)

	// 1つ目のキー: 深さ2へ進む。
	_, shouldClick, advanced := s.SelectKey(spatial.LabelKeys[0])
	if shouldClick {
		t.Fatal("at depth 1 (< 2 stages) should not click")
	}
	if !advanced {
		t.Fatal("should advance to depth 2")
	}

	// 2つ目のキーは最終段(深さ2)でクリックが発生するはず。
	_, shouldClick, _ = s.SelectKey(spatial.LabelKeys[0])
	if !shouldClick {
		t.Fatal("at last stage should click")
	}
}
