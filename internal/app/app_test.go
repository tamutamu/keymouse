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
	shift    bool
	moved    [][2]int
	clicks   []spatial.ClickAction
	moveErr  error
	clickErr error
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
func (m *mockOverlay) SetKeyHandler(fn func(uintptr))                   {}

type mockOverlayFactory struct{ ov *mockOverlay }

func (m *mockOverlayFactory) NewOverlay(mon monitor.Info, size spatial.LabelSize) (Overlay, error) {
	return m.ov, nil
}

// newTestApp は既定設定とモック依存で App を構築するヘルパー。
func newTestApp(t *testing.T) (*App, *mockInput, *mockOverlay) {
	t.Helper()
	in := &mockInput{}
	ov := &mockOverlay{}
	deps := Deps{
		Input:          in,
		Monitors:       &mockMonitors{info: monitor.Info{Left: 0, Top: 0, Right: 1920, Bottom: 1080}},
		OverlayFactory: &mockOverlayFactory{ov: ov},
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
	if a.session.State() != session.StateIdle {
		t.Fatalf("expected Idle after click, got %s", a.session.State())
	}
}

func TestLabelKeyAdvancesWithoutClicking(t *testing.T) {
	a, in, ov := newTestApp(t)
	a.onHotkey(spatial.ClickLeft) // 1920×1080 の予定表は3段
	a.onKeyDown(uintptr(spatial.LabelKeys[0]))

	if len(in.clicks) != 0 {
		t.Fatalf("should not click at depth 1 of a 3-stage schedule, got %v", in.clicks)
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
