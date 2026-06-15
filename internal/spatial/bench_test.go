package spatial

import (
	"testing"
	"time"
)

// BenchmarkInitialLabels は最初のオーバーレイ用のアンカー生成を計測する。
// 設計目標: p95 < 100ms（重い処理は GDI 描画であってアンカー計算ではないため、
// このベンチマークは純粋な CPU 部分のみを切り出している）。
func BenchmarkInitialLabels(b *testing.B) {
	display := Rect{X: 0, Y: 0, W: 1920, H: 1080}
	source := display
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateAnchors(5, 5, display, source, LabelKeys)
	}
}

// BenchmarkRefinement は以降の絞り込み段階（ユーザーがラベルキーを押した後）の
// アンカー生成を計測する。設計目標: p95 < 50ms。
func BenchmarkRefinement(b *testing.B) {
	display := Rect{X: 0, Y: 0, W: 1920, H: 1080}
	source := display
	initial := GenerateAnchors(5, 5, display, source, LabelKeys)
	// 最初のラベルを押した状況を模擬: そのセルの領域へ絞り込む。
	refined := initial[0].SourceArea
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateAnchors(5, 5, display, refined, LabelKeys)
	}
}

// BenchmarkFullDepthChain はモニターサイズから算出した予定表の全段を辿る時間を計測する。
func BenchmarkFullDepthChain(b *testing.B) {
	cfg := DefaultConfig()
	display := Rect{X: 0, Y: 0, W: 1920, H: 1080}
	schedule := GridSchedule(display.W, display.H, cfg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := display
		for _, g := range schedule {
			anchors := GenerateAnchors(g[0], g[1], display, src, LabelKeys)
			src = anchors[0].SourceArea
		}
	}
}

// TestInitialLabelsBudget は、低速なハードウェアでも単一のアンカー生成呼び出しが
// 100ms 目標を十分に下回って完了することを保証する。
// これはベンチマークではなく、所要時間に関するアサーションである。
func TestInitialLabelsBudget(t *testing.T) {
	const budget = 10 * time.Millisecond          // Go の GC ジッターに対し 10 倍の安全マージン
	display := Rect{X: 0, Y: 0, W: 3840, H: 2160} // 最悪ケース: 4K
	source := display

	start := time.Now()
	GenerateAnchors(5, 5, display, source, LabelKeys)
	elapsed := time.Since(start)

	if elapsed > budget {
		t.Errorf("initial label generation took %v, want < %v", elapsed, budget)
	}
}

// TestRefinementBudget は絞り込み段階が 50ms の予算内に収まることを保証する。
func TestRefinementBudget(t *testing.T) {
	const budget = 5 * time.Millisecond // 10 倍の安全マージン
	display := Rect{X: 0, Y: 0, W: 3840, H: 2160}
	source := display
	initial := GenerateAnchors(5, 5, display, source, LabelKeys)
	refined := initial[0].SourceArea

	start := time.Now()
	GenerateAnchors(5, 5, display, refined, LabelKeys)
	elapsed := time.Since(start)

	if elapsed > budget {
		t.Errorf("refinement took %v, want < %v", elapsed, budget)
	}
}
