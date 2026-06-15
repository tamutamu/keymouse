package spatial

import (
	"math"
	"testing"
)

// TestDeterminism は同一の入力で GenerateAnchors を2回呼び出したとき、
// 完全に同一の出力が得られる（乱数やグローバル状態の変化がない）ことを検証する。
func TestDeterminism(t *testing.T) {
	display := Rect{X: 0, Y: 0, W: 1920, H: 1080}
	source := Rect{X: 0, Y: 0, W: 1920, H: 1080}
	labels := LabelKeys

	first := GenerateAnchors(5, 5, display, source, labels)
	second := GenerateAnchors(5, 5, display, source, labels)

	if len(first) != len(second) {
		t.Fatalf("determinism: length mismatch %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("determinism: anchor[%d] differs: %+v vs %+v", i, first[i], second[i])
		}
	}
}

// TestGenerateAnchors はアンカー生成の基本的な性質を確認する。
//   - 25個のラベルに対してちょうど25個のアンカーが生成される
//   - 各アンカーが正しいラベルを持つ
//   - どの2つのアンカーも DisplayRect が重ならない（互いに素である）
//   - すべての SourcePoint が sourceArea 内に収まる
func TestGenerateAnchors(t *testing.T) {
	display := Rect{X: 0, Y: 0, W: 500, H: 500}
	source := Rect{X: 0, Y: 0, W: 1000, H: 1000}
	labels := LabelKeys // 25 keys

	anchors := GenerateAnchors(5, 5, display, source, labels)

	// ちょうど25個のアンカーを期待する（デフォルトの 5×5 グリッド、25ラベル）。
	if len(anchors) != 25 {
		t.Fatalf("expected 25 anchors, got %d", len(anchors))
	}

	// ラベルが一致することを確認する。
	for i, a := range anchors {
		if a.Label != labels[i] {
			t.Errorf("anchor[%d]: expected label %v, got %v", i, labels[i], a.Label)
		}
	}

	// すべての SourcePoint が sourceArea 内にあることを確認する。
	for i, a := range anchors {
		if !source.Contains(a.SourcePoint) {
			t.Errorf("anchor[%d]: SourcePoint %+v outside sourceArea %+v", i, a.SourcePoint, source)
		}
	}

	// DisplayRect が互いに重ならない（表示領域をタイル状に敷き詰める）ことを確認する。
	for i := range anchors {
		for j := i + 1; j < len(anchors); j++ {
			if rectsOverlap(anchors[i].DisplayRect, anchors[j].DisplayRect) {
				t.Errorf("anchors[%d] and anchors[%d] overlap: %+v and %+v",
					i, j, anchors[i].DisplayRect, anchors[j].DisplayRect)
			}
		}
	}

	// DisplayRect の和集合が表示領域とちょうど一致することを確認する。
	totalArea := 0.0
	for _, a := range anchors {
		totalArea += a.DisplayRect.W * a.DisplayRect.H
	}
	expected := display.W * display.H
	if math.Abs(totalArea-expected) > 1e-6 {
		t.Errorf("total DisplayRect area %.6f does not equal display area %.6f", totalArea, expected)
	}
}

// TestBackspace は SelectionStage の push/pop（スタック動作）を模擬する。
// spatial パッケージ自体はスタックを保持しないが、ズームインした段階のアンカーが
// 親段階の領域内に厳密に収まることを検証できる。
func TestBackspace(t *testing.T) {
	// 段階1: 全画面。
	display := Rect{X: 0, Y: 0, W: 1000, H: 1000}
	source := Rect{X: 0, Y: 0, W: 2000, H: 2000}

	stage1 := GenerateAnchors(5, 5, display, source, LabelKeys)

	if len(stage1) == 0 {
		t.Fatal("stage1 produced no anchors")
	}

	// 最初のアンカーを選び、その領域へズームインする。
	chosen := stage1[0]
	stage2 := GenerateAnchors(5, 5, chosen.DisplayRect, chosen.SourceArea, LabelKeys)

	// 段階2の各アンカーの SourcePoint は chosen.SourceArea 内になければならない。
	for i, a := range stage2 {
		if !chosen.SourceArea.Contains(a.SourcePoint) {
			t.Errorf("stage2 anchor[%d] SourcePoint %+v outside parent SourceArea %+v",
				i, a.SourcePoint, chosen.SourceArea)
		}
	}

	// 段階2の各アンカーの DisplayRect は chosen.DisplayRect 内になければならない。
	for i, a := range stage2 {
		if !rectContainsRect(chosen.DisplayRect, a.DisplayRect) {
			t.Errorf("stage2 anchor[%d] DisplayRect %+v outside parent DisplayRect %+v",
				i, a.DisplayRect, chosen.DisplayRect)
		}
	}

	// 「Backspace」の模擬: 段階2を破棄しても段階1は無傷のまま。
	// 段階1を再生成し、元と完全に同一であることを確認する。
	stage1b := GenerateAnchors(5, 5, display, source, LabelKeys)
	if len(stage1) != len(stage1b) {
		t.Fatalf("backspace: stage1 length changed after zoom (%d vs %d)", len(stage1), len(stage1b))
	}
	for i := range stage1 {
		if stage1[i] != stage1b[i] {
			t.Errorf("backspace: stage1 anchor[%d] mutated after zoom: %+v vs %+v",
				i, stage1[i], stage1b[i])
		}
	}
}

// TestCoordinateConversion は複数の表示／ソースの組み合わせにわたって、
// オーバーレイ座標から物理座標へのマッピング精度を確認する。
func TestCoordinateConversion(t *testing.T) {
	cases := []struct {
		name    string
		display Rect
		source  Rect
	}{
		{"1:1 mapping", Rect{0, 0, 100, 100}, Rect{0, 0, 100, 100}},
		{"2x scale", Rect{0, 0, 100, 100}, Rect{0, 0, 200, 200}},
		{"offset source", Rect{0, 0, 100, 100}, Rect{500, 300, 200, 200}},
		{"non-square display", Rect{0, 0, 1920, 1080}, Rect{0, 0, 3840, 2160}},
		{"partial display", Rect{10, 20, 500, 400}, Rect{100, 200, 1000, 800}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			anchors := GenerateAnchors(5, 5, tc.display, tc.source, LabelKeys)

			for i, a := range anchors {
				// SourcePoint は SourceArea 内になければならない。
				if !a.SourceArea.Contains(a.SourcePoint) {
					t.Errorf("anchor[%d]: SourcePoint %+v outside SourceArea %+v",
						i, a.SourcePoint, a.SourceArea)
				}

				// SourcePoint は SourceArea の中心と一致するはず。
				center := a.SourceArea.Center()
				if math.Abs(a.SourcePoint.X-center.X) > 1e-9 ||
					math.Abs(a.SourcePoint.Y-center.Y) > 1e-9 {
					t.Errorf("anchor[%d]: SourcePoint %+v != SourceArea.Center() %+v",
						i, a.SourcePoint, center)
				}

				// 物理座標への丸めによって、点がソース外へ 1px を超えてずれてはならない。
				px, py := SourcePointPhysical(a)
				dx := math.Abs(float64(px) - a.SourcePoint.X)
				dy := math.Abs(float64(py) - a.SourcePoint.Y)
				if dx > 1.0 || dy > 1.0 {
					t.Errorf("anchor[%d]: SourcePointPhysical (%d,%d) too far from %.3f,%.3f",
						i, px, py, a.SourcePoint.X, a.SourcePoint.Y)
				}

				// DisplayRect は displayArea 内になければならない（浮動小数点の許容誤差込み）。
				const eps = 1e-6
				dr := a.DisplayRect
				if dr.X < tc.display.X-eps || dr.Y < tc.display.Y-eps ||
					dr.X+dr.W > tc.display.X+tc.display.W+eps ||
					dr.Y+dr.H > tc.display.Y+tc.display.H+eps {
					t.Errorf("anchor[%d]: DisplayRect %+v outside display %+v", i, dr, tc.display)
				}
			}
		})
	}
}

// --- ヘルパー -----------------------------------------------------------------

// rectsOverlap は2つの矩形が内部領域を共有する場合に true を返す。
// 辺のみを共有する隣接矩形は重なりとは見なさない。
func rectsOverlap(a, b Rect) bool {
	const eps = 1e-9
	return a.X+a.W-eps > b.X &&
		b.X+b.W-eps > a.X &&
		a.Y+a.H-eps > b.Y &&
		b.Y+b.H-eps > a.Y
}

// rectContainsRect は inner が outer の内側に完全に収まる場合に true を返す（許容誤差込み）。
func rectContainsRect(outer, inner Rect) bool {
	const eps = 1e-9
	return inner.X >= outer.X-eps &&
		inner.Y >= outer.Y-eps &&
		inner.X+inner.W <= outer.X+outer.W+eps &&
		inner.Y+inner.H <= outer.Y+outer.H+eps
}
