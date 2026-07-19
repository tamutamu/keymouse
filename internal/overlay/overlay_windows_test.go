//go:build windows

package overlay

import (
	"testing"

	"github.com/tamutamu/keymouse/internal/spatial"
)

func TestStageFontSizeIsStableAcrossAnchorSizes(t *testing.T) {
	o := &Overlay{labelSize: spatial.LabelNormal}
	want := fontSizeMap[spatial.LabelNormal]
	for _, size := range []float64{4, 12, 48, 200} {
		o.anchors = []spatial.Anchor{{DisplayRect: spatial.Rect{W: size, H: size}}}
		if got := o.stageFontSize(); got != want {
			t.Fatalf("cell size %.0f: font=%d, want %d", size, got, want)
		}
	}
}
