package target

import (
	"testing"

	"github.com/tamutamu/keymouse/internal/spatial"
)

func TestNormalizeActionableKeepsNestedIndependentControls(t *testing.T) {
	area := spatial.Rect{X: 0, Y: 0, W: 1920, H: 1080}
	resultLink := Target{
		Name: "Search result", Role: RoleHyperlink, Enabled: true,
		Bounds:  spatial.Rect{X: 186, Y: 233, W: 576, H: 122},
		Actions: []Action{ActionClick},
	}
	translateLink := Target{
		Name: "Translate", Role: RoleHyperlink, Enabled: true,
		Bounds:  spatial.Rect{X: 437, Y: 268, W: 169, H: 32},
		Actions: []Action{ActionClick},
	}

	got := NormalizeActionable([]Target{resultLink, translateLink}, area, 0)
	if len(got) != 2 {
		t.Fatalf("nested independent controls: got %d targets, want 2", len(got))
	}
}

func TestNormalizeActionableMergesNearEqualWrappers(t *testing.T) {
	area := spatial.Rect{X: 0, Y: 0, W: 1920, H: 1080}
	targets := []Target{
		{Name: "Save", Role: RoleButton, Enabled: true, Bounds: spatial.Rect{X: 100, Y: 100, W: 100, H: 40}, Actions: []Action{ActionClick}},
		{Name: "Save wrapper", Role: RoleUnknown, Enabled: true, Bounds: spatial.Rect{X: 102, Y: 101, W: 98, H: 39}, Actions: []Action{ActionClick}},
	}

	got := NormalizeActionable(targets, area, 0)
	if len(got) != 1 {
		t.Fatalf("near-equal wrappers: got %d targets, want 1", len(got))
	}
	if got[0].Role != RoleButton {
		t.Fatalf("semantic target should win: got role %q", got[0].Role)
	}
}
