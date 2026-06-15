package spatial

import (
	"math"
	"testing"
)

// テスト対象の解像度一覧。
var testResolutions = []struct {
	name string
	w, h float64
}{
	{"FullHD", 1920, 1080},
	{"WQHD", 2560, 1440},
	{"4K", 3840, 2160},
}

// TestAnchorSourcePointsInMonitorRect は、すべての対象解像度において
// 各アンカーの SourcePoint がモニター矩形内に収まることを検証する。
func TestAnchorSourcePointsInMonitorRect(t *testing.T) {
	for _, res := range testResolutions {
		t.Run(res.name, func(t *testing.T) {
			display := Rect{X: 0, Y: 0, W: res.w, H: res.h}
			anchors := GenerateAnchors(5, 5, display, display, LabelKeys)
			for _, a := range anchors {
				if !display.Contains(a.SourcePoint) {
					t.Errorf("label %v: SourcePoint %v outside monitor %v", a.Label, a.SourcePoint, display)
				}
			}
		})
	}
}

// TestAnchorSourcePointsInMonitorRectMultiMonitor は、セカンダリモニター
// （原点が0でない）でも、そのモニターの矩形内に収まる座標が生成されることを検証する。
func TestAnchorSourcePointsInMonitorRectMultiMonitor(t *testing.T) {
	for _, res := range testResolutions {
		t.Run(res.name+"_secondary", func(t *testing.T) {
			// プライマリの右側にあるセカンダリモニターを模擬する。
			display := Rect{X: res.w, Y: 0, W: res.w, H: res.h}
			source := display
			anchors := GenerateAnchors(5, 5, display, source, LabelKeys)
			for _, a := range anchors {
				if !display.Contains(a.SourcePoint) {
					t.Errorf("label %v: SourcePoint %v outside secondary monitor %v", a.Label, a.SourcePoint, display)
				}
			}
		})
	}
}

// TestAnchorDeterminism は、同一の入力が常に同一のアンカー配置を生成すること
// （同じキー列が同じピクセルに到達すること）を検証する。
func TestAnchorDeterminism(t *testing.T) {
	for _, res := range testResolutions {
		t.Run(res.name, func(t *testing.T) {
			display := Rect{X: 0, Y: 0, W: res.w, H: res.h}
			a1 := GenerateAnchors(5, 5, display, display, LabelKeys)
			a2 := GenerateAnchors(5, 5, display, display, LabelKeys)
			for i := range a1 {
				if a1[i].SourcePoint != a2[i].SourcePoint {
					t.Errorf("non-deterministic: label %v idx %d: %v != %v",
						a1[i].Label, i, a1[i].SourcePoint, a2[i].SourcePoint)
				}
			}
		})
	}
}

// TestAnchorCoverageNoGap は、アンカーセルが隙間なくモニター全域をタイル状に
// 敷き詰めること（すべての DisplayRect の和集合が表示矩形を覆うこと）を検証する。
func TestAnchorCoverageNoGap(t *testing.T) {
	for _, res := range testResolutions {
		t.Run(res.name, func(t *testing.T) {
			display := Rect{X: 0, Y: 0, W: res.w, H: res.h}
			anchors := GenerateAnchors(5, 5, display, display, LabelKeys)

			// すべての表示セルの合計面積はモニター面積と一致しなければならない。
			var totalArea float64
			for _, a := range anchors {
				totalArea += a.DisplayRect.W * a.DisplayRect.H
			}
			expected := display.W * display.H
			if math.Abs(totalArea-expected) > 1e-6 {
				t.Errorf("cell area sum %.2f != monitor area %.2f (delta %.6f)",
					totalArea, expected, totalArea-expected)
			}
		})
	}
}

// TestRefinementCoordChain は、すべての解像度において予定表の全段を辿っても、
// 各段階で座標が元のモニター内に収まり続けることを検証する。
func TestRefinementCoordChain(t *testing.T) {
	cfg := DefaultConfig()
	for _, res := range testResolutions {
		t.Run(res.name, func(t *testing.T) {
			display := Rect{X: 0, Y: 0, W: res.w, H: res.h}
			schedule := GridSchedule(display.W, display.H, cfg)
			src := display
			for depth, g := range schedule {
				anchors := GenerateAnchors(g[0], g[1], display, src, LabelKeys)
				if len(anchors) == 0 {
					t.Fatalf("depth %d: no anchors generated", depth+1)
				}
				for _, a := range anchors {
					if !display.Contains(a.SourcePoint) {
						t.Errorf("depth %d label %v: SourcePoint %v outside monitor",
							depth+1, a.Label, a.SourcePoint)
					}
				}
				// 最初のセルへ降りる。
				src = anchors[0].SourceArea
			}
		})
	}
}

// TestPhysicalCoordRounding は、SourcePointPhysical が丸め後にモニター境界の
// 外側の座標を決して生成しないことを検証する。
func TestPhysicalCoordRounding(t *testing.T) {
	for _, res := range testResolutions {
		t.Run(res.name, func(t *testing.T) {
			display := Rect{X: 0, Y: 0, W: res.w, H: res.h}
			anchors := GenerateAnchors(5, 5, display, display, LabelKeys)
			for _, a := range anchors {
				px, py := SourcePointPhysical(a)
				if float64(px) < display.X || float64(px) >= display.X+display.W ||
					float64(py) < display.Y || float64(py) >= display.Y+display.H {
					t.Errorf("label %v: physical (%d,%d) outside monitor %v", a.Label, px, py, display)
				}
			}
		})
	}
}

// TestGridScheduleResolutionScaling は、高解像度ほど 5×5 がより深い段まで維持され、
// 生成される段数が解像度とともに増える（少なくとも減らない）ことを検証する。
func TestGridScheduleResolutionScaling(t *testing.T) {
	cfg := DefaultConfig()
	fhd := GridSchedule(1920, 1080, cfg)
	uhd := GridSchedule(3840, 2160, cfg)

	if len(fhd) == 0 || len(uhd) == 0 {
		t.Fatal("schedule must have at least one stage")
	}
	if len(uhd) < len(fhd) {
		t.Errorf("4K should allow at least as many stages as FullHD: 4K=%d FullHD=%d", len(uhd), len(fhd))
	}
	// 最初の段はどの解像度でも希望の最大グリッド(5×5)になるはず。
	if fhd[0] != [2]int{5, 5} || uhd[0] != [2]int{5, 5} {
		t.Errorf("first stage should be 5x5, got FullHD=%v 4K=%v", fhd[0], uhd[0])
	}
}
