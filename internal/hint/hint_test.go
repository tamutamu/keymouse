package hint

import (
	"github.com/tamutamu/keymouse/internal/target"
	"testing"
)

func TestAssignAndFilter(t *testing.T) {
	ts := make([]target.Target, 11)
	items := Assign(ts)
	if len(items) != 11 || len(items[0].Label) != 1 || len(items[9].Label) != 2 {
		t.Fatalf("items=%v", items)
	}
	if got := Filter(items, items[0].Label); len(got) != 1 {
		t.Fatalf("prefix A: %d", len(got))
	}
}
