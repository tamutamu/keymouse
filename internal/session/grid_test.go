package session

import (
	"testing"

	"github.com/tamutamu/keymouse/internal/spatial"
)

func TestFixedGridLabelsClickOnThirdKey(t *testing.T) {
	s := New(spatial.DefaultConfig(), spatial.GridKeys)
	if !s.Start(spatial.ClickLeft, spatial.Rect{W: 1800, H: 1200}) {
		t.Fatal("start")
	}
	anchors := s.CurrentAnchors()
	if len(anchors) != 1000 {
		t.Fatalf("anchors=%d", len(anchors))
	}
	if spatial.Label3ToStr(anchors[0].Label) != "aaa" || spatial.Label3ToStr(anchors[1].Label) != "aas" {
		t.Fatal("labels are not row-major base-ten")
	}
	for i, k := range anchors[999].Label {
		_, click, _ := s.SelectKey(k)
		if i < 2 && click {
			t.Fatal("clicked early")
		}
	}
	if s.State() != StateExecuting {
		t.Fatal("third key must execute")
	}
}

func TestPanClearsPartialInput(t *testing.T) {
	s := New(spatial.DefaultConfig(), spatial.GridKeys)
	s.Start(spatial.ClickLeft, spatial.Rect{W: 1800, H: 1200})
	s.SelectKey(spatial.GridKeys[0])
	s.Pan(4, 0)
	if s.CurrentDepth() != 0 {
		t.Fatal("pan must clear input")
	}
	if s.CurrentAnchors()[0].SourcePoint.X != 26.5 {
		t.Fatal("pan must move click target")
	}
}
