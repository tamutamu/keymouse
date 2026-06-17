package app

import (
	"errors"
	"testing"

	"github.com/tamutamu/keymouse/internal/monitor"
	"github.com/tamutamu/keymouse/internal/session"
	"github.com/tamutamu/keymouse/internal/settings"
	"github.com/tamutamu/keymouse/internal/spatial"
)

// 以下は各ポートのモック実装。インターフェース抽象化により、
// app のオーケストレーションを Win32 なしで(=非 Windows でも)テストできる。

type mockInput struct {
	shift         bool
	alt           bool
	shiftReleased bool
	moved         [][2]int
	clicks        []spatial.ClickAction
	moveErr       error
	clickErr      error
}

func (m *mockInput) MoveCursor(x, y int) error {
	m.moved = append(m.moved, [2]int{x, y})
	return m.moveErr
}
func (m *mockInput) Click(a spatial.ClickAction) error {
	m.clicks = append(m.clicks, a)
	return m.clickErr
}
func (m *mockInput) ShiftHeld() bool { return m.shift }
func (m *mockInput) AltHeld() bool   { return m.alt }
func (m *mockInput) ReleaseShift()   { m.shiftReleased = true }

type mockMonitors struct {
	info monitor.Info
	err  error
}

func (m *mockMonitors) MonitorAtCursor() (monitor.Info, error) { return m.info, m.err }

type mockOverlay struct {
	shown     bool
	destroyed bool
	updates   int
}

func (m *mockOverlay) Show(a []spatial.Anchor, act spatial.ClickAction) { m.shown = true }
func (m *mockOverlay) UpdateAnchors(a []spatial.Anchor)                 { m.updates++ }
func (m *mockOverlay) Hide()                                            { m.shown = false }
func (m *mockOverlay) Destroy()                                         { m.destroyed = true }

type mockOverlayFactory struct{ ov *mockOverlay }

func (m *mockOverlayFactory) NewOverlay(mon monitor.Info, size spatial.LabelSize) (Overlay, error) {
	return m.ov, nil
}

type mockHook struct {
	installed bool
	onKey     func(vk uint32, down bool) bool
}

func (m *mockHook) Install(onKey func(vk uint32, down bool) bool) error {
	m.installed = true
	m.onKey = onKey
	return nil
}
func (m *mockHook) Remove() { m.installed = false }

// newTestApp は既定設定とモック依存で App を構築するヘルパー。
func newTestApp(t *testing.T) (*App, *mockInput, *mockOverlay) {
	t.Helper()
	in := &mockInput{}
	ov := &mockOverlay{}
	deps := Deps{
		Input:          in,
		Monitors:       &mockMonitors{info: monitor.Info{Left: 0, Top: 0, Right: 1920, Bottom: 1080}},
		OverlayFactory: &mockOverlayFactory{ov: ov},
		Hook:           &mockHook{},
	}
	return newApp(settings.Default(), deps), in, ov
}

func TestOnHotkeyStartsSelecting(t *testing.T) {
	a, _, ov := newTestApp(t)
	a.onHotkey(spatial.ClickLeft)

	if a.session.State() != session.StateSelecting {
		t.Fatalf("expected Selecting, got %s", a.session.State())
	}
	if !ov.shown {
		t.Fatal("overlay should be shown")
	}
	if len(a.session.CurrentAnchors()) != 25 {
		t.Fatalf("expected 25 anchors, got %d", len(a.session.CurrentAnchors()))
	}
}

func TestOnHotkeyMonitorErrorAborts(t *testing.T) {
	a, _, ov := newTestApp(t)
	a.deps.Monitors = &mockMonitors{err: errors.New("no monitor")}

	a.onHotkey(spatial.ClickLeft)
	if a.session.State() != session.StateIdle {
		t.Fatalf("expected Idle after monitor error, got %s", a.session.State())
	}
	if ov.shown {
		t.Fatal("overlay should not be shown when monitor lookup fails")
	}
}

func TestShiftKeyClicksImmediately(t *testing.T) {
	a, in, _ := newTestApp(t)
	a.onHotkey(spatial.ClickRight)
	in.shift = true

	a.onKeyDown(uintptr(spatial.LabelKeys[0]))

	if len(in.clicks) != 1 || in.clicks[0] != spatial.ClickRight {
		t.Fatalf("expected one right click, got %v", in.clicks)
	}
	if len(in.moved) != 1 {
		t.Fatalf("expected cursor move, got %v", in.moved)
	}
	// Shift+ラベルの即クリックでは、クリックに Shift が漏れないよう解除されるべき。
	if !in.shiftReleased {
		t.Fatal("Shift should be released before clicking")
	}
	if a.session.State() != session.StateIdle {
		t.Fatalf("expected Idle after click, got %s", a.session.State())
	}
}

func TestLabelKeyAdvancesWithoutClicking(t *testing.T) {
	a, in, ov := newTestApp(t)
	a.onHotkey(spatial.ClickLeft) // 1920×1080 の予定表は複数段(最終段以外では即クリックしない)
	a.onKeyDown(uintptr(spatial.LabelKeys[0]))

	if len(in.clicks) != 0 {
		t.Fatalf("should not click at depth 1 of a multi-stage schedule, got %v", in.clicks)
	}
	if ov.updates != 1 {
		t.Fatalf("overlay should refresh once, got %d", ov.updates)
	}
	if a.session.CurrentDepth() != 2 {
		t.Fatalf("expected depth 2, got %d", a.session.CurrentDepth())
	}
}

func TestEscCancelsWithoutClick(t *testing.T) {
	a, in, ov := newTestApp(t)
	a.onHotkey(spatial.ClickLeft)

	a.onKeyDown(vkEscape)
	if len(in.clicks) != 0 {
		t.Fatal("Esc must not click")
	}
	if len(in.moved) != 0 {
		t.Fatal("Esc must not move cursor")
	}
	if a.session.State() != session.StateIdle {
		t.Fatalf("expected Idle after Esc, got %s", a.session.State())
	}
	if !ov.destroyed {
		t.Fatal("overlay should be destroyed on cancel")
	}
}

func TestBackspaceGoesBack(t *testing.T) {
	a, _, ov := newTestApp(t)
	a.onHotkey(spatial.ClickLeft)
	a.onKeyDown(uintptr(spatial.LabelKeys[0])) // depth 2
	if a.session.CurrentDepth() != 2 {
		t.Fatalf("setup: expected depth 2, got %d", a.session.CurrentDepth())
	}

	ovUpdatesBefore := ov.updates
	a.onKeyDown(vkBack)
	if a.session.CurrentDepth() != 1 {
		t.Fatalf("expected depth 1 after Backspace, got %d", a.session.CurrentDepth())
	}
	if ov.updates != ovUpdatesBefore+1 {
		t.Fatal("overlay should refresh after Backspace")
	}
}

func TestShiftDoubleTapOpensOverlay(t *testing.T) {
	a, _, ov := newTestApp(t)

	// Shift の新規押下→離上→新規押下(短時間)でオーバーレイが開く。
	a.onKeyHook(vkShift, true)
	a.onKeyHook(vkShift, false)
	a.onKeyHook(vkShift, true)

	if a.session.State() != session.StateSelecting {
		t.Fatalf("expected Selecting after double Shift tap, got %s", a.session.State())
	}
	if !ov.shown {
		t.Fatal("overlay should be shown after double Shift tap")
	}
}

func TestSingleShiftDoesNotOpen(t *testing.T) {
	a, _, ov := newTestApp(t)
	a.onKeyHook(vkShift, true)
	a.onKeyHook(vkShift, false)
	if a.session.State() != session.StateIdle || ov.shown {
		t.Fatal("single Shift tap must not open overlay")
	}
}

func TestShiftThenOtherKeyDoesNotOpen(t *testing.T) {
	a, _, ov := newTestApp(t)
	a.onKeyHook(vkShift, true)  // 1打目
	a.onKeyHook(vkShift, false) //
	a.onKeyHook(0x41, true)     // 別キー(A)で連打シーケンスを破棄
	a.onKeyHook(0x41, false)
	a.onKeyHook(vkShift, true) // これは新たな1打目扱い
	if a.session.State() != session.StateIdle || ov.shown {
		t.Fatal("Shift, other key, Shift must not open overlay")
	}
}

func TestShiftAutoRepeatDoesNotOpen(t *testing.T) {
	a, _, ov := newTestApp(t)
	// Shift 押しっぱなしのオートリピート(離上なしの連続 down)は1打目のまま。
	a.onKeyHook(vkShift, true)
	a.onKeyHook(vkShift, true)
	a.onKeyHook(vkShift, true)
	if a.session.State() != session.StateIdle || ov.shown {
		t.Fatal("auto-repeat Shift must not open overlay")
	}
}

func TestAltPanShiftsGridWithoutSelecting(t *testing.T) {
	a, in, ov := newTestApp(t)
	a.onHotkey(spatial.ClickLeft)
	in.alt = true

	x0 := a.session.CurrentAnchors()[0].SourcePoint.X
	depth0 := a.session.CurrentDepth()
	updates0 := ov.updates

	a.onKeyDown(vkL) // Alt+l → 右へパン

	// パンは段を変えない(選択ではない)。
	if a.session.CurrentDepth() != depth0 {
		t.Fatalf("pan must not change depth: %d -> %d", depth0, a.session.CurrentDepth())
	}
	// クリック先(元座標)が右へ panStepPx ずれる。
	x1 := a.session.CurrentAnchors()[0].SourcePoint.X
	if x1 != x0+panStepPx {
		t.Fatalf("expected source X to shift by %.0f (%.3f -> %.3f)", panStepPx, x0, x1)
	}
	if ov.updates != updates0+1 {
		t.Fatalf("overlay should refresh once after pan, got %d", ov.updates-updates0)
	}

	// Alt 押下中の移動キー以外(ここでは Alt+A=ラベルキー)は無視され、絞り込まない。
	a.onKeyDown(uintptr(spatial.LabelKeys[0]))
	if a.session.CurrentDepth() != depth0 {
		t.Fatalf("Alt+label must be ignored, depth changed to %d", a.session.CurrentDepth())
	}
}
