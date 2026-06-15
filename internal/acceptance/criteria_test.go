// Package acceptance は tasks.md §11.4 に記載された受け入れ基準の自動検証を行う。
// 実際のディスプレイを必要とする基準（オーバーレイ描画、実際のクリック発行など）は、
// 手動確認としてコード中に注記している。
package acceptance

import (
	"testing"

	"github.com/tamutamu/keymouse/internal/session"
	"github.com/tamutamu/keymouse/internal/spatial"
)

// ヘルパー ---------------------------------------------------------------

// fhd は 1920×1080 のモニター矩形。MinLabelPx=20 のとき予定表は3段になる。
var fhd = spatial.Rect{X: 0, Y: 0, W: 1920, H: 1080}

// newSes は安全上限 maxDepth を持つセッションを生成する。
func newSes(maxDepth int) *session.Session {
	cfg := spatial.Config{Cols: 5, Rows: 5, MinLabelPx: 20, MaxDepth: maxDepth}
	return session.New(cfg, spatial.LabelKeys)
}

// startSes は Idle から選択を開始する(Start が初期段を構築し Selecting へ遷移する)。
func startSes(t *testing.T, s *session.Session, action spatial.ClickAction) {
	t.Helper()
	if !s.Start(action, fhd) {
		t.Fatal("Start failed")
	}
}

// Criterion 1: グリッド線・十字線を描画しない（ラベルのみ、グリッド線なし）
// 手動確認: keymouse.exe を起動し、Alt+L を押して、グリッドや十字線が表示されず、
// 薄暗くなった実画面の上に半透明の文字ラベルだけが重なって表示されることを確認する。

// Criterion 2: Shift+key で任意段階にてクリック
// 任意の深さで Shift+key を押すと即座にクリックが実行されることを検証する。
func TestCriteria02_ShiftKeyClickAtAnyDepth(t *testing.T) {
	// 1920×1080 の予定表は3段。各深さ(1〜3)で Shift+key が即クリックすることを確認する。
	for depth := 1; depth <= 3; depth++ {
		s := newSes(8)
		startSes(t, s, spatial.ClickLeft)

		// Shift なしで (depth-1) 段階だけ掘り下げる。
		for i := 1; i < depth; i++ {
			key := spatial.LabelKeys[i%len(spatial.LabelKeys)]
			_, shouldClick, advanced := s.SelectKey(key)
			if shouldClick {
				t.Fatalf("depth %d step %d: unexpected early click", depth, i)
			}
			if !advanced {
				t.Fatalf("depth %d step %d: expected advance, key=%v", depth, i, key)
			}
		}

		// ここで Shift+key は深さに関わらずクリックを発火する。
		key := spatial.LabelKeys[0]
		anchor, ok := s.SelectKeyWithClick(key)
		if !ok {
			t.Fatalf("depth %d: SelectKeyWithClick failed for key %v", depth, key)
		}
		if anchor.Label != key {
			t.Fatalf("depth %d: wrong anchor label %v, want %v", depth, anchor.Label, key)
		}
		if s.State() != session.StateExecuting {
			t.Fatalf("depth %d: expected Executing after Shift+key, got %s", depth, s.State())
		}
		s.FinishExecuting()
		if s.State() != session.StateIdle {
			t.Fatalf("depth %d: expected Idle after FinishExecuting, got %s", depth, s.State())
		}
	}
}

// Criterion 3: Backspace で前段階に戻る
// Backspace で1段階ずつ親段階へ戻れること、最小深さ（1）ではそれ以上戻れないことを検証する。
func TestCriteria03_BackspaceReturnsToParentStage(t *testing.T) {
	s := newSes(8)
	startSes(t, s, spatial.ClickLeft)

	// 2段階分掘り下げる(予定表は3段なので、深さ3まで進める)。
	s.SelectKey(spatial.LabelKeys[0])
	if s.CurrentDepth() != 2 {
		t.Fatalf("expected depth 2, got %d", s.CurrentDepth())
	}
	s.SelectKey(spatial.LabelKeys[1])
	if s.CurrentDepth() != 3 {
		t.Fatalf("expected depth 3, got %d", s.CurrentDepth())
	}

	// Backspace 1回 → 深さ2。
	if !s.Backspace() {
		t.Fatal("Backspace from depth 3 should succeed")
	}
	if s.CurrentDepth() != 2 {
		t.Fatalf("after Backspace: expected depth 2, got %d", s.CurrentDepth())
	}

	// もう一度 Backspace → 深さ1（最小、これ以上は pop しない）。
	if !s.Backspace() {
		t.Fatal("Backspace from depth 2 should succeed")
	}
	if s.CurrentDepth() != 1 {
		t.Fatalf("after 2nd Backspace: expected depth 1, got %d", s.CurrentDepth())
	}

	// 深さ1での Backspace は false を返すはず。
	if s.Backspace() {
		t.Fatal("Backspace at depth 1 should return false")
	}
	if s.CurrentDepth() != 1 {
		t.Fatalf("depth should remain 1, got %d", s.CurrentDepth())
	}
}

// Criterion 4: Esc でキャンセル、カーソル位置を変更しない
// Cancel() の意味論: 状態を Idle に戻し、カーソルを移動させない。
func TestCriteria04_EscCancelNoStateLeaks(t *testing.T) {
	s := newSes(8)
	startSes(t, s, spatial.ClickRight)

	s.SelectKey(spatial.LabelKeys[2])
	s.Cancel()

	if s.State() != session.StateIdle {
		t.Fatalf("after Cancel: expected Idle, got %s", s.State())
	}
	if s.CurrentAnchors() != nil {
		t.Fatal("anchors should be nil after Cancel")
	}
	if s.ClickAction() != 0 {
		t.Fatalf("action should be zero after Cancel, got %d", s.ClickAction())
	}
	// 手動確認: 画面上のカーソル位置が変化していないことを確認する。
}

// Criterion 5: IME / Caps Lock 非依存
// 設計上、本アプリは Win32 仮想キーコード（WM_KEYDOWN の wParam）を読み取っており、
// IME 変換や Caps Lock の状態の影響を受けない。本テストは Key 型が文字コードではなく
// uint16 の VK コードであることを検証する。
func TestCriteria05_LabelKeysAreVKCodes(t *testing.T) {
	for _, k := range spatial.LabelKeys {
		// A-Z の VK コードは 0x41-0x5A（65-90）。
		vk := uint16(k)
		if vk < 0x41 || vk > 0x5A {
			t.Errorf("LabelKey %04X is outside VK A-Z range", vk)
		}
	}
}

// Criterion 6: 画像をファイル保存・外部送信しない
// 設計判断により画面キャプチャ自体を廃止した(オーバーレイは実画面を透過表示する)。
// したがって画面ピクセルを読む箇所が一切存在しない。
// 手動確認（または CI）: `grep -rn "BitBlt\|GetDIBits\|os.WriteFile\|http" internal/` を実行 → 一致0件。

// Criterion 7: セッション進行中の多重起動を無視
// 選択中に再度 Start を呼んでも無視され、状態と元のクリック種別が維持されることを検証する。
func TestCriteria07_DuplicateSessionIgnored(t *testing.T) {
	s := newSes(8)
	startSes(t, s, spatial.ClickLeft)

	// 選択中の2回目の Start は false を返し、元のクリック種別を維持しなければならない。
	if s.Start(spatial.ClickRight, fhd) {
		t.Fatal("second Start should be ignored")
	}
	if s.ClickAction() != spatial.ClickLeft {
		t.Fatal("action should remain ClickLeft after ignored Start")
	}
	if s.State() != session.StateSelecting {
		t.Fatalf("state should remain Selecting, got %s", s.State())
	}
}

// Criterion 8: 最終段に達したら自動クリック（追加キー不要）
// 安全上限 MaxDepth=2 で予定表を2段に絞り、最終段のキー選択で自動クリックが
// 発火することを検証する。
func TestCriteria08_AutoClickAtLastStage(t *testing.T) {
	s := newSes(2) // 予定表は2段に制限される
	startSes(t, s, spatial.ClickLeft)

	// 深さ1 → 2: 進むだけでクリックしないはず。
	_, shouldClick, advanced := s.SelectKey(spatial.LabelKeys[0])
	if shouldClick {
		t.Fatal("should not click at depth 1 (2-stage schedule)")
	}
	if !advanced {
		t.Fatal("should advance at depth 1")
	}

	// 深さ2 = 最終段: 次のキーはクリックを発火しなければならない。
	_, shouldClick, _ = s.SelectKey(spatial.LabelKeys[0])
	if !shouldClick {
		t.Fatal("should click at last stage")
	}
	if s.State() != session.StateExecuting {
		t.Fatalf("expected Executing at last stage, got %s", s.State())
	}
}

// Criterion 9: 複数モニター対応 — カーソルのあるモニターを対象にする
// 手動確認: 2台目のモニターを接続し、モニター2上にカーソルを置いて Alt+L を押し、
// オーバーレイがモニター2に表示されることを確認する。

// Criterion 10: 状態機械の正確な遷移
// Idle → Selecting → Executing → Idle の遷移が正しく行われることを検証する。
func TestCriteria10_StateMachineTransitions(t *testing.T) {
	s := newSes(3)

	// Idle → Selecting（Start が初期段を構築する）。
	if !s.Start(spatial.ClickLeft, fhd) {
		t.Fatal("Start from Idle")
	}
	if s.State() != session.StateSelecting {
		t.Fatalf("expected Selecting, got %s", s.State())
	}

	// Selecting → Executing（SelectKeyWithClick 経由）。
	_, ok := s.SelectKeyWithClick(spatial.LabelKeys[0])
	if !ok {
		t.Fatal("SelectKeyWithClick should succeed")
	}
	if s.State() != session.StateExecuting {
		t.Fatalf("expected Executing, got %s", s.State())
	}

	// Executing → Idle。
	s.FinishExecuting()
	if s.State() != session.StateIdle {
		t.Fatalf("expected Idle, got %s", s.State())
	}
}

// Criterion 11: セッション終了時に資源を破棄する
// 画面キャプチャを廃止したため保持・破棄すべき画像は存在しない。オーバーレイは
// セッション終了時(クリック/キャンセル/パニック)に App.cleanup で必ず破棄される。
// app_test.go の TestEscCancelsWithoutClick 等でオーバーレイ破棄を検証済み。

// Criterion 12: パニック発生時にオーバーレイを破棄して Idle に戻る
// onHotkey と onKeyDown 内の recover() ブロックが cancelSession() を呼ぶ。
// app_windows.go のコード検査により確認済み。

// Criterion 13: 待機中（Idle）に画面キャプチャ・高頻度ポーリングなし
// Idle 経路には goroutine、ticker、ポーリングループが存在しない。
// アプリはイベント駆動の RunMessageLoop（GetMessage）でブロックする。
// 手動確認: Process Hacker やタスクマネージャーで、待機時の CPU 使用率が 0.1% 未満であることを確認する。

// Criterion 14: ラベルサイズ切替（小/標準/大/特大）
// 4種類のラベルサイズ定数がすべて異なり、かつ正の値であることを検証する。
func TestCriteria14_LabelSizeConstants(t *testing.T) {
	sizes := []spatial.LabelSize{
		spatial.LabelSmall,
		spatial.LabelNormal,
		spatial.LabelLarge,
		spatial.LabelXLarge,
	}
	unique := map[spatial.LabelSize]bool{}
	for _, s := range sizes {
		if unique[s] {
			t.Errorf("duplicate LabelSize value %d", s)
		}
		unique[s] = true
		if s <= 0 {
			t.Errorf("LabelSize %d must be > 0", s)
		}
	}
	if len(unique) != 4 {
		t.Errorf("expected 4 distinct sizes, got %d", len(unique))
	}
	// 手動確認: 設定を開いてラベルサイズを変更し、オーバーレイを再表示して
	// 文字が目に見えて大きく／小さくなることを確認する。
}
