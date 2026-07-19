package app

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tamutamu/keymouse/internal/hint"
	"github.com/tamutamu/keymouse/internal/input"
	"github.com/tamutamu/keymouse/internal/monitor"
	"github.com/tamutamu/keymouse/internal/session"
	"github.com/tamutamu/keymouse/internal/settings"
	"github.com/tamutamu/keymouse/internal/spatial"
	"github.com/tamutamu/keymouse/internal/target"
	"github.com/tamutamu/keymouse/internal/uia"
)

const (
	vkEscape           = 0x1B
	vkBack             = 0x08
	vkSpace            = 0x20
	vkShift            = 0x10
	vkLShift           = 0xA0
	vkRShift           = 0xA1
	vkH                = 0x48
	vkJ                = 0x4A
	vkK                = 0x4B
	vkL                = 0x4C
	vkR                = 0x52
	vkG                = 0x47
	panStepPx          = 4.0
	panShiftMultiplier = 4.0
)
const continuousRefreshDelay = 100 * time.Millisecond
const uiaDiscoveryTimeout = 6 * time.Second

func isShiftVK(v uint32) bool { return v == vkShift || v == vkLShift || v == vkRShift }
func isModifierVK(v uint32) bool {
	return isShiftVK(v) || v == 0x11 || v == 0xA2 || v == 0xA3 || v == 0x12 || v == 0xA4 || v == 0xA5
}

type App struct {
	cfg                  settings.Config
	session              *session.Session
	deps                 Deps
	overlay              Overlay
	shiftDown            bool
	continuous           bool
	peeking              bool
	area                 spatial.Rect
	peekVK               uint32
	elementHints         []hint.Item
	elementPrefix        string
	elementAction        spatial.ClickAction
	elementLabelOffset   spatial.Point
	requestActivation    func(spatial.ClickAction, bool, bool)
	requestExecution     func(executionRequest)
	activationGeneration atomic.Uint64
	activationPending    bool
	labelRefreshDown     bool
	modeToggleDown       bool
	gridMode             bool
}

type activationPlan struct {
	action     spatial.ClickAction
	area       spatial.Rect
	items      []hint.Item
	grid       bool
	err        error
	continuous bool
}

type executionRequest struct {
	generation uint64
	action     spatial.ClickAction
	continuous bool
	element    *target.Target
	anchor     *spatial.Anchor
}

type executionResult struct {
	request executionRequest
	err     error
	next    activationPlan
	area    spatial.Rect
}

func newApp(cfg settings.Config, deps Deps) *App {
	a := &App{cfg: cfg, session: session.New(spatial.DefaultConfig(), spatial.GridKeys), deps: deps, continuous: true, peekVK: peekVirtualKey(cfg.PeekKey)}
	a.requestActivation = a.onHotkey
	a.requestExecution = func(request executionRequest) { a.applyExecution(a.performExecution(request)) }
	return a
}

func peekVirtualKey(name string) uint32 {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "" || name == "SPACE" {
		return vkSpace
	}
	if len(name) == 1 && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= '0' && name[0] <= '9')) {
		candidate := uint32(name[0])
		// A peek key must never shadow a label key; doing so would make every
		// anchor containing that key impossible to select.
		if !spatial.IsGridKey(spatial.Key(candidate)) {
			return candidate
		}
	}
	return vkSpace
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

func (a *App) onHotkey(action spatial.ClickAction, continuous, gridMode bool) {
	a.gridMode = gridMode
	a.applyActivation(a.prepareActivation(action, continuous, gridMode))
}

// prepareActivation performs monitor and UIA discovery without creating any
// thread-affine windows. It is safe to run on a worker COM apartment.
func (a *App) prepareActivation(action spatial.ClickAction, continuous, gridMode bool) activationPlan {
	r, e := a.target()
	if e != nil {
		return activationPlan{action: action, continuous: continuous, err: e}
	}
	plan := activationPlan{action: action, area: r, continuous: continuous}
	if !gridMode {
		items, err := a.discoverElementHints()
		if err != nil {
			plan.err = err
			return plan
		}
		if len(items) > 0 {
			plan.items = items
			return plan
		}
		plan.err = fmt.Errorf("UIA discovery returned no visible actionable elements")
		return plan
	}
	plan.grid = true
	return plan
}

func (a *App) showLoadingOverlay() {
	r, err := a.target()
	if err != nil {
		return
	}
	ov, err := a.deps.OverlayFactory.NewOverlay(r, a.cfg.LabelSize)
	if err != nil {
		return
	}
	a.overlay = ov
	a.area = r
	ov.ShowLoading()
	a.deps.Input.HideCursor()
}

// applyActivation runs on the app/message thread because Overlay owns HWNDs.
func (a *App) applyActivation(plan activationPlan) {
	if plan.err != nil {
		log.Printf("selection unavailable: %v", plan.err)
		a.session.Finish()
		a.cleanup()
		return
	}
	// Replace any prior selection atomically on the app thread.
	a.session.Finish()
	a.cleanup()
	a.continuous = plan.continuous
	if len(plan.items) > 0 {
		a.showNewElementOverlay(plan.action, plan.area, plan.items)
		return
	}
	if !plan.grid {
		return
	}
	r, action := plan.area, plan.action
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
	a.area = r
	a.peeking = false
	ov.Show(a.session.VisibleAnchors(), a.session.CurrentDepth(), action)
	a.deps.Input.HideCursor()
}

func (a *App) discoverElementHints() ([]hint.Item, error) {
	targets, err := uia.IsolatedDiscoverForeground(uiaDiscoveryTimeout)
	if err != nil {
		return nil, err
	}
	area, areaErr := a.target()
	if areaErr != nil {
		return nil, areaErr
	}
	actionable := target.NormalizeActionable(targets, area, hint.MaxLabel3Items)
	return hint.Assign(actionable), nil
}
func (a *App) onKeyHook(v uint32, down bool) bool {
	if down && v == vkEscape {
		a.activationGeneration.Add(1)
		wasPending := a.activationPending
		a.activationPending = false
		if wasPending {
			a.session.Finish()
			a.cleanup()
			return true
		}
		if a.session.State() == session.StateExecuting {
			a.session.Finish()
			a.cleanup()
			return true
		}
	}
	if isShiftVK(v) {
		if down {
			a.shiftDown = true
		} else {
			a.shiftDown = false
		}
	}
	// Shift+G toggles between Element and Grid modes while selection is active.
	// Consume key repeat and both edges; unshifted g remains a label key.
	if v == vkG && !down && a.modeToggleDown {
		a.modeToggleDown = false
		return true
	}
	if down && v == vkG && a.shiftDown {
		active := a.activationPending || a.overlay != nil || a.session.State() == session.StateSelecting || len(a.elementHints) > 0
		if active {
			if !a.modeToggleDown {
				a.modeToggleDown = true
				action := a.elementAction
				if len(a.elementHints) == 0 {
					action = a.session.ClickAction()
				}
				a.requestActivation(action, a.continuous, !a.gridMode)
			}
			return true
		}
	}
	// Shift+R refreshes the currently displayed labels. Consume both edges and
	// suppress key-repeat so one physical press starts only one UIA scan.
	if v == vkR && !down && a.labelRefreshDown {
		a.labelRefreshDown = false
		return true
	}
	if down && v == vkR && a.shiftDown {
		active := a.overlay != nil || a.session.State() == session.StateSelecting || len(a.elementHints) > 0
		if active {
			if !a.labelRefreshDown {
				a.labelRefreshDown = true
				action := a.elementAction
				if len(a.elementHints) == 0 {
					action = a.session.ClickAction()
				}
				a.requestActivation(action, a.continuous, a.gridMode)
			}
			return true
		}
	}
	// During selection Space is normally consumed by Peek before RegisterHotKey
	// can emit WM_HOTKEY. Handle the stop side of Shift+Space here, ahead of Peek.
	if down && v == vkSpace && a.shiftDown {
		active := a.activationPending || a.overlay != nil || a.session.State() != session.StateIdle || len(a.elementHints) > 0
		if active {
			a.toggleContinuousActivation()
			return true
		}
	}
	if len(a.elementHints) > 0 {
		return a.onElementKey(v, down)
	}
	if a.session.State() == session.StateSelecting {
		if v == a.peekVK {
			if down && !a.peeking {
				a.peeking = true
				if a.overlay != nil {
					a.overlay.Hide()
				}
			} else if !down && a.peeking {
				a.peeking = false
				if a.overlay != nil {
					a.overlay.Show(a.session.VisibleAnchors(), a.session.CurrentDepth(), a.session.ClickAction())
				}
			}
			return true
		}
		if a.peeking {
			if down && v == vkEscape {
				a.cancelSession()
			}
			return !isModifierVK(v)
		}
		if down && !isModifierVK(v) {
			a.onKeyDown(uintptr(v))
		}
		return !isModifierVK(v)
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
		if a.shiftDown {
			return
		}
		if !input.IsLabelKey(v) {
			return
		}
		anchor, click, advanced := a.session.SelectKey(spatial.Key(v))
		if click {
			a.beginGridExecution(anchor)
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
		a.overlay.UpdateAnchors(a.session.VisibleAnchors(), a.session.CurrentDepth())
	}
}
func (a *App) beginGridExecution(anchor spatial.Anchor) {
	a.deps.Input.RestoreCursor()
	if a.overlay != nil {
		a.overlay.Hide()
	}
	generation := a.activationGeneration.Add(1)
	a.requestExecution(executionRequest{generation: generation, action: a.session.ClickAction(), continuous: a.continuous, anchor: &anchor})
}

func (a *App) performExecution(request executionRequest) executionResult {
	result := executionResult{request: request}
	a.deps.Input.ReleaseShift()
	if request.element != nil {
		// UI Automation is discovery-only in the interactive hint workflow.
		// Always perform a real mouse click at the displayed target position.
		p := request.element.Bounds.Center()
		if result.err = a.deps.Input.MoveCursor(int(p.X+0.5), int(p.Y+0.5)); result.err == nil {
			result.err = a.deps.Input.Click(request.action)
		}
	} else if request.anchor != nil {
		x, y := spatial.SourcePointPhysical(*request.anchor)
		if result.err = a.deps.Input.MoveCursor(x, y); result.err == nil {
			result.err = a.deps.Input.Click(request.action)
		}
	}
	if result.err != nil || !request.continuous {
		return result
	}
	time.Sleep(continuousRefreshDelay)
	if request.element != nil {
		for attempt := 0; attempt < 5; attempt++ {
			if request.generation != a.activationGeneration.Load() {
				return result
			}
			result.next = a.prepareActivation(request.action, true, false)
			if result.next.err == nil {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
	} else {
		result.area, result.err = a.target()
	}
	return result
}

func (a *App) applyExecution(result executionResult) {
	if result.request.generation != a.activationGeneration.Load() {
		return
	}
	if result.err != nil || !result.request.continuous {
		if result.request.anchor != nil {
			a.session.Finish()
		}
		a.cleanup()
		return
	}
	if result.request.element != nil {
		a.elementHints = nil
		a.applyActivation(result.next)
		return
	}
	r, err := result.area, result.err
	if err != nil || !a.session.Continue(r) {
		a.session.Finish()
		a.cleanup()
		return
	}
	a.peeking = false
	if a.overlay == nil || r != a.area {
		if a.overlay != nil {
			a.overlay.Destroy()
		}
		a.overlay, err = a.deps.OverlayFactory.NewOverlay(r, a.cfg.LabelSize)
		if err != nil {
			a.session.Finish()
			a.cleanup()
			return
		}
	}
	a.area = r
	a.overlay.Show(a.session.VisibleAnchors(), 0, a.session.ClickAction())
	a.deps.Input.HideCursor()
}
func (a *App) cancelSession() {
	a.peeking = false
	a.session.Cancel()
	a.cleanup()
}
func (a *App) cleanup() {
	a.deps.Input.RestoreCursor()
	a.peeking = false
	a.elementHints = nil
	a.elementPrefix = ""
	if a.overlay != nil {
		a.overlay.Destroy()
		a.overlay = nil
	}
}

func (a *App) toggleContinuousActivation() {
	active := a.activationPending || a.overlay != nil || a.session.State() != session.StateIdle || len(a.elementHints) > 0
	if active {
		a.activationGeneration.Add(1)
		a.activationPending = false
		a.session.Finish()
		a.cleanup()
		return
	}
	// Every fresh Shift+Space activation starts in Element mode.
	a.requestActivation(spatial.ClickLeft, true, false)
}

func (a *App) startElementSelection(action spatial.ClickAction, area spatial.Rect) (bool, error) {
	items, err := a.discoverElementHints()
	if err != nil {
		return false, err
	}
	if len(items) == 0 {
		return false, nil
	}
	return a.showNewElementOverlay(action, area, items), nil
}

func (a *App) showNewElementOverlay(action spatial.ClickAction, area spatial.Rect, items []hint.Item) bool {
	a.elementLabelOffset = spatial.Point{}
	anchors := anchorsForHints(items, a.elementLabelOffset)
	if a.overlay != nil {
		a.overlay.Destroy()
	}
	ov, err := a.deps.OverlayFactory.NewOverlay(area, a.cfg.LabelSize)
	if err != nil {
		return false
	}
	a.overlay, a.area, a.peeking = ov, area, false
	a.elementHints, a.elementPrefix, a.elementAction = items, "", action
	ov.Show(anchors, 0, action)
	a.deps.Input.HideCursor()
	return true
}

func anchorsForHints(items []hint.Item, offset spatial.Point) []spatial.Anchor {
	if len(items) > hint.MaxLabel3Items {
		items = items[:hint.MaxLabel3Items]
	}
	r := make([]spatial.Anchor, 0, len(items))
	for _, item := range items {
		var label spatial.Label3
		for i := 0; i < len(item.Label) && i < 3; i++ {
			label[i] = labelVirtualKey(item.Label[i])
		}
		b := item.Target.Bounds
		b.X += offset.X
		b.Y += offset.Y
		r = append(r, spatial.Anchor{Label: label, DisplayRect: b, SourceArea: b, SourcePoint: b.Center()})
	}
	return r
}

// Hint labels are stored as lowercase display text, while spatial.Label3
// stores Windows virtual-key values (A=0x41 ... Z=0x5A). Keeping this
// conversion at the UI boundary prevents invisible labels.
func labelVirtualKey(ch byte) spatial.Key {
	if ch >= 'a' && ch <= 'z' {
		ch -= 'a' - 'A'
	}
	return spatial.Key(ch)
}

func (a *App) onElementKey(v uint32, down bool) bool {
	if v == a.peekVK {
		if down && !a.peeking {
			a.peeking = true
			a.overlay.Hide()
		}
		if !down && a.peeking {
			a.peeking = false
			a.showElementHints()
		}
		return true
	}
	if a.peeking {
		if down && v == vkEscape {
			a.cleanup()
		}
		return !isModifierVK(v)
	}
	if !down {
		return !isModifierVK(v)
	}
	if v == vkEscape {
		a.cleanup()
		return true
	}
	if v == vkBack {
		if len(a.elementPrefix) > 0 {
			a.elementPrefix = a.elementPrefix[:len(a.elementPrefix)-1]
			a.showElementHints()
		}
		return true
	}
	if v == vkH || v == vkJ || v == vkK || v == vkL {
		a.moveElementLabels(uintptr(v))
		return true
	}
	if !spatial.IsGridKey(spatial.Key(v)) {
		return !isModifierVK(v)
	}
	// Label selection accepts unshifted (lowercase) keys only. Shift remains
	// reserved for refresh and accelerated H/J/K/L movement.
	if a.shiftDown {
		return true
	}
	a.elementPrefix += spatial.KeyToChar(spatial.Key(v))
	visible := hint.Filter(a.elementHints, a.elementPrefix)
	if len(visible) == 1 && visible[0].Label == a.elementPrefix {
		a.beginElementExecution(visible[0].Target)
		return true
	}
	if len(visible) == 0 || len(a.elementPrefix) >= 3 {
		a.elementPrefix = ""
	}
	a.showElementHints()
	return true
}

func (a *App) showElementHints() {
	if a.overlay == nil || a.peeking {
		return
	}
	a.overlay.Show(anchorsForHints(hint.Filter(a.elementHints, a.elementPrefix), a.elementLabelOffset), len(a.elementPrefix), a.elementAction)
	a.deps.Input.HideCursor()
}

func (a *App) moveElementLabels(v uintptr) {
	step := panStepPx
	if a.shiftDown {
		step *= panShiftMultiplier
	}
	switch v {
	case vkH:
		a.elementLabelOffset.X -= step
	case vkJ:
		a.elementLabelOffset.Y += step
	case vkK:
		a.elementLabelOffset.Y -= step
	case vkL:
		a.elementLabelOffset.X += step
	}
	a.showElementHints()
}

func (a *App) beginElementExecution(t target.Target) {
	a.deps.Input.RestoreCursor()
	if a.overlay != nil {
		a.overlay.Hide()
	}
	// Keep the execution rectangle aligned with the moved label. The UIA
	// element identity is retained only to select the element refresh path;
	// performExecution never invokes a UIA control pattern.
	t.Bounds.X += a.elementLabelOffset.X
	t.Bounds.Y += a.elementLabelOffset.Y
	generation := a.activationGeneration.Add(1)
	a.requestExecution(executionRequest{generation: generation, action: a.elementAction, continuous: a.continuous, element: &t})
}
