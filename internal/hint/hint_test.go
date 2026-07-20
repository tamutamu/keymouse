package hint

import (
	"testing"

	"github.com/tamutamu/keymouse/internal/target"
)

func TestAssignAndFilter(t *testing.T) {
	ts := make([]target.Target, len(Keys)+1)
	items := Assign(ts)
	if len(items) != len(ts) || len(items[0].Label) != 1 || len(items[len(Keys)-1].Label) != 2 {
		t.Fatalf("items=%v", items)
	}
	if got := Filter(items, items[0].Label); len(got) != 1 {
		t.Fatalf("prefix A: %d", len(got))
	}
}

func TestAssignUsesOneKeyForEveryAvailableElementKey(t *testing.T) {
	items := Assign(make([]target.Target, len(Keys)))
	for _, item := range items {
		if len(item.Label) != 1 {
			t.Fatalf("label %q is not one key", item.Label)
		}
	}
}
