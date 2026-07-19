package target

import (
	"math"
	"sort"

	"github.com/tamutamu/keymouse/internal/spatial"
)

func NormalizeActionable(input []Target, area spatial.Rect, limit int) []Target {
	candidates := make([]Target, 0, len(input))
	for _, t := range input {
		if t.Actionable() && area.Contains(t.Bounds.Center()) {
			candidates = append(candidates, t)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		pi, pj := targetPriority(candidates[i]), targetPriority(candidates[j])
		if pi != pj {
			return pi > pj
		}
		return candidates[i].Bounds.W*candidates[i].Bounds.H < candidates[j].Bounds.W*candidates[j].Bounds.H
	})
	selected := make([]Target, 0, len(candidates))
	for _, candidate := range candidates {
		duplicate := false
		for _, existing := range selected {
			if sameIdentity(candidate, existing) || nearDuplicateBounds(candidate.Bounds, existing.Bounds) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			selected = append(selected, candidate)
		}
		if limit > 0 && len(selected) == limit {
			break
		}
	}
	sort.SliceStable(selected, func(i, j int) bool {
		a, b := selected[i], selected[j]
		if math.Abs(a.Bounds.Y-b.Bounds.Y) > 4 {
			return a.Bounds.Y < b.Bounds.Y
		}
		if a.Bounds.X != b.Bounds.X {
			return a.Bounds.X < b.Bounds.X
		}
		if a.Bounds.Y != b.Bounds.Y {
			return a.Bounds.Y < b.Bounds.Y
		}
		if a.ControlType != b.ControlType {
			return a.ControlType < b.ControlType
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
	return selected
}

// nearDuplicateBounds identifies provider wrappers that describe effectively
// the same visual control. A high overlap alone is insufficient: Google search
// result hyperlinks contain smaller independent controls such as "Translate"
// and result-menu buttons. Using the smaller area as the sole denominator
// incorrectly removed the containing title link. Require comparable sizes too.
func nearDuplicateBounds(a, b spatial.Rect) bool {
	areaA, areaB := a.W*a.H, b.W*b.H
	if areaA <= 0 || areaB <= 0 {
		return false
	}
	smaller, larger := math.Min(areaA, areaB), math.Max(areaA, areaB)
	return smaller/larger >= 0.70 && overlapRatio(a, b) >= 0.85
}

func targetPriority(t Target) int {
	score := 0
	switch t.Role {
	case RoleHyperlink, RoleButton, RoleEdit, RoleCheckBox, RoleComboBox,
		RoleMenuItem, RoleRadio, RoleTabItem, RoleTreeItem, RoleListItem:
		// Prefer semantic controls when Chrome exposes a semantic element and
		// an overlapping Custom/Group/Legacy wrapper for the same hit area.
		score += 50
	case RoleUnknown:
		// Unknown controls remain eligible when Invoke availability or a
		// LegacyIAccessible default action proves they are clickable.
	}
	for _, action := range t.Actions {
		switch action {
		case ActionInvoke, ActionToggle, ActionSelect:
			score += 100
		case ActionFocus:
			score += 20
		case ActionClick:
			score += 10
		}
	}
	if t.Name != "" {
		score += 5
	}
	if t.AutomationID != "" {
		score += 2
	}
	return score
}
func sameIdentity(a, b Target) bool {
	return a.Bounds == b.Bounds && a.ControlType == b.ControlType && a.Name == b.Name && a.AutomationID == b.AutomationID
}
func overlapRatio(a, b spatial.Rect) float64 {
	x1, y1 := math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	x2, y2 := math.Min(a.X+a.W, b.X+b.W), math.Min(a.Y+a.H, b.Y+b.H)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	denominator := math.Min(a.W*a.H, b.W*b.H)
	if denominator <= 0 {
		return 0
	}
	return (x2 - x1) * (y2 - y1) / denominator
}
