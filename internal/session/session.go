// Package session manages a fixed three-key grid selection.
package session

import (
	"github.com/tamutamu/keymouse/internal/spatial"
	"sync"
)

type State int

const (
	StateIdle State = iota
	StateSelecting
	StateExecuting
	StateCancelled
)

func (s State) String() string { return [...]string{"Idle", "Selecting", "Executing", "Cancelled"}[s] }

// Session owns one 40x25 grid. Labels never change: the third valid key
// executes the click at that cell's centre.
type Session struct {
	mu      sync.Mutex
	state   State
	action  spatial.ClickAction
	area    spatial.Rect
	anchors []spatial.Anchor
	input   []spatial.Key
}

func New(_ spatial.Config, _ []spatial.Key) *Session { return &Session{} }
func (s *Session) State() State                      { s.mu.Lock(); defer s.mu.Unlock(); return s.state }
func (s *Session) ClickAction() spatial.ClickAction {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.action
}
func (s *Session) CurrentDepth() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.input) }
func (s *Session) Start(action spatial.ClickAction, area spatial.Rect) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateIdle || area.W <= 0 || area.H <= 0 {
		return false
	}
	s.action = action
	s.area = area
	s.input = nil
	s.anchors = spatial.GenerateGridAnchors(area)
	s.state = StateSelecting
	return true
}
func (s *Session) CurrentAnchors() []spatial.Anchor {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateSelecting {
		return nil
	}
	return append([]spatial.Anchor(nil), s.anchors...)
}
func (s *Session) SelectKey(k spatial.Key) (spatial.Anchor, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateSelecting || !spatial.IsGridKey(k) {
		return spatial.Anchor{}, false, false
	}
	s.input = append(s.input, k)
	if len(s.input) < 3 {
		return spatial.Anchor{}, false, true
	}
	label := spatial.Label3{s.input[0], s.input[1], s.input[2]}
	for _, a := range s.anchors {
		if a.Label == label {
			s.state = StateExecuting
			return a, true, false
		}
	}
	s.input = nil
	return spatial.Anchor{}, false, false
}
func (s *Session) Pan(dx, dy float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateSelecting {
		return false
	}
	s.area.X += dx
	s.area.Y += dy
	for i := range s.anchors {
		a := &s.anchors[i]
		a.DisplayRect.X += dx
		a.DisplayRect.Y += dy
		a.SourceArea.X += dx
		a.SourceArea.Y += dy
		a.SourcePoint.X += dx
		a.SourcePoint.Y += dy
	}
	s.input = nil
	return true
}
func (s *Session) Backspace() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateSelecting || len(s.input) == 0 {
		return false
	}
	s.input = s.input[:len(s.input)-1]
	return true
}
func (s *Session) Cancel()          { s.mu.Lock(); defer s.mu.Unlock(); s.resetLocked() }
func (s *Session) FinishExecuting() { s.mu.Lock(); defer s.mu.Unlock(); s.resetLocked() }
func (s *Session) resetLocked()     { s.state = StateIdle; s.action = 0; s.input = nil; s.anchors = nil }
