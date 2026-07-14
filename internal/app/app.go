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

const (
	vkEscape           = 0x1B
	vkBack             = 0x08
	vkShift            = 0x10
	vkLShift           = 0xA0
	vkRShift           = 0xA1
	vkH                = 0x48
	vkJ                = 0x4A
	vkK                = 0x4B
	vkL                = 0x4C
	panStepPx          = 4.0
	panShiftMultiplier = 4.0
)
const shiftDoubleTapWindow = 400 * time.Millisecond

func isShiftVK(v uint32) bool { return v == vkShift || v == vkLShift || v == vkRShift }
func isModifierVK(v uint32) bool {
	return isShiftVK(v) || v == 0x11 || v == 0xA2 || v == 0xA3 || v == 0x12 || v == 0xA4 || v == 0xA5
}

type App struct {
	cfg       settings.Config
	session   *session.Session
	deps      Deps
	overlay   Overlay
	shiftTap  *input.DoubleTap
	shiftDown bool
}

func newApp(cfg settings.Config, deps Deps) *App {
	return &App{cfg: cfg, session: session.New(spatial.DefaultConfig(), spatial.GridKeys), deps: deps, shiftTap: input.NewDoubleTap(shiftDoubleTapWindow)}
}
func rectFromMonitor(m monitor.Info) spatial.Rect {
	return spatial.Rect{X: float64(m.Left), Y: float64(m.Top), W: float64(m.Width()), H: float64(m.Height())}
}

// target is the full cursor monitor.  Keeping the labels and the overlay in
// the same coordinate space is essential for scanning two-key labels quickly.
func (a *App) target() (spatial.Rect, error) {
	m, e := a.deps.Monitors.MonitorAtCursor()
	if e != nil {
		return spatial.Rect{}, e
	}
	return rectFromMonitor(m), nil
}

func (a *App) onHotkey(action spatial.ClickAction) {
	r, e := a.target()
	if e != nil {
		log.Printf("selection target: %v", e)
		return
	}
	if !a.session.Start(action, r) {
		return
	}
	// The grid and overlay use the same full-monitor rectangle.
	if a.overlay != nil {
		a.overlay.Destroy()
	}
	ov, e := a.deps.OverlayFactory.NewOverlay(r, a.cfg.LabelSize)
	if e != nil {
		a.session.Cancel()
		return
	}
	a.overlay = ov
	ov.Show(a.session.CurrentAnchors(), action)
}
func (a *App) onKeyHook(v uint32, down bool) bool {
	freshShift := false
	if isShiftVK(v) {
		if down {
			freshShift = !a.shiftDown
			a.shiftDown = true
		} else {
			a.shiftDown = false
		}
	}
	if a.session.State() == session.StateSelecting {
		if down && !isModifierVK(v) {
			a.onKeyDown(uintptr(v))
		}
		return !isModifierVK(v)
	}
	if freshShift && a.shiftTap.Tap(time.Now()) {
		a.onHotkey(spatial.ClickLeft)
	}
	if down && !isModifierVK(v) {
		a.shiftTap.Reset()
	}
	return false
}
func (a *App) onKeyDown(v uintptr) {
	if a.session.State() != session.StateSelecting {
		return
	}
	switch v {
	case vkEscape:
		a.cancelSession()
	case vkBack:
		if a.session.Backspace() {
			a.refresh()
		}
	case vkH, vkJ, vkK, vkL:
		a.panGrid(v)
	default:
		if !input.IsLabelKey(v) {
			return
		}
		anchor, click, advanced := a.session.SelectKey(spatial.Key(v))
		if click {
			a.executeClick(anchor)
		} else if advanced {
			a.refresh()
		}
	}
}
func (a *App) panGrid(v uintptr) {
	step := panStepPx
	if a.shiftDown {
		step *= panShiftMultiplier
	}
	var dx, dy float64
	switch v {
	case vkH:
		dx = -step
	case vkJ:
		dy = step
	case vkK:
		dy = -step
	case vkL:
		dx = step
	}
	if a.session.Pan(dx, dy) {
		a.refresh()
	}
}
func (a *App) refresh() {
	if a.overlay != nil {
		a.overlay.UpdateAnchors(a.session.CurrentAnchors())
	}
}
func (a *App) executeClick(anchor spatial.Anchor) {
	if a.overlay != nil {
		a.overlay.Hide()
	}
	a.deps.Input.ReleaseShift()
	x, y := spatial.SourcePointPhysical(anchor)
	if e := a.deps.Input.MoveCursor(x, y); e == nil {
		_ = a.deps.Input.Click(a.session.ClickAction())
	}
	a.session.FinishExecuting()
	a.cleanup()
}
func (a *App) cancelSession() { a.session.Cancel(); a.cleanup() }
func (a *App) cleanup() {
	if a.overlay != nil {
		a.overlay.Destroy()
		a.overlay = nil
	}
}
