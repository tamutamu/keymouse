package spatial

// Config は階層的なグリッド選択のパラメータを保持する。
//
// グリッドの列数・行数は「希望する最大グリッド」であり、実際に各段で使う
// グリッドは GridSchedule がモニターサイズと MinLabelPx から自動決定する。
// これにより、低解像度では浅い段で自動的に粗いグリッドへ縮小し、高解像度では
// 5×5 をより深い段まで維持する(ズームに頼らずラベルの可読性を保つ)。
type Config struct {
	Cols       int     // 希望する最大グリッド列数(ラベルは A〜Y の25個なので最大5)
	Rows       int     // 希望する最大グリッド行数(同上)
	MinLabelPx float64 // ラベルが読める最小セル一辺のピクセル数(これを下回らないよう分割を抑える)
	MaxDepth   int     // 段数の安全上限(暴走防止)
}

// DefaultConfig はデフォルト構成(最大5×5、最小ラベル12px、最大3段)を返す。
//
// MinLabelPx は最終段のセルの最小辺長を実質的に決める。値を小さくするほど最終セルが
// 小さくなり、カーソルが飛ぶ到達点が細かくなる(クリック可能な箇所が増える)が、その分
// ラベルも小さくなる。ラベルはオーバーレイ側でセルに合わせて自動縮小・グリッド線で
// 区切られるため、12px 程度まで下げても判読できる。
func DefaultConfig() Config {
	return Config{
		Cols:       5,
		Rows:       5,
		MinLabelPx: 12,
		MaxDepth:   3,
	}
}

// GridSchedule はモニターサイズ(物理ピクセル)から、各段で用いるグリッド
// (列数, 行数)の並びを算出する。
//
// 重要な性質: ある段の領域サイズは「どのセルを選んだか」に依存せず、それまでの
// グリッド列だけで一意に決まる(c×r で割れば必ず 1/c × 1/r)。したがって
// セッション開始時にモニターサイズから予定表を一度だけ計算できる。
//
// 各段では、セル一辺が MinLabelPx を下回らない範囲で、その段の最大グリッドまでの
// グリッドを選ぶ。最大グリッドは gridCapAtDepth により段が深いほど小さくなる
// (浅い段は密に素早く絞り、深い段は粗く読みやすく)。両方向とも1セルしか取れなく
// なった時点で打ち切る(それ以上は読める大きさに分割できない)。最後の段でキーを
// 押すとクリックが実行される。
func GridSchedule(monW, monH float64, cfg Config) [][2]int {
	minPx := cfg.MinLabelPx
	if minPx <= 0 {
		minPx = 12
	}
	maxCols := cfg.Cols
	if maxCols <= 0 {
		maxCols = 5
	}
	maxRows := cfg.Rows
	if maxRows <= 0 {
		maxRows = 5
	}
	maxDepth := cfg.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	var schedule [][2]int
	w, h := monW, monH
	for len(schedule) < maxDepth {
		depth := len(schedule)
		colCap := gridCapAtDepth(depth, maxCols)
		rowCap := gridCapAtDepth(depth, maxRows)
		cols := clampInt(int(w/minPx), 1, colCap)
		rows := clampInt(int(h/minPx), 1, rowCap)
		if cols == 1 && rows == 1 {
			break // これ以上は読めるサイズに分割できない。
		}
		schedule = append(schedule, [2]int{cols, rows})
		w /= float64(cols)
		h /= float64(rows)
	}
	if len(schedule) == 0 {
		// モニターが極端に小さい場合でも最低1段は用意する。
		schedule = append(schedule, [2]int{1, 1})
	}
	return schedule
}

// gridCapAtDepth は段の深さ(0始まり)に応じた最大グリッド(片軸)を返す。
// 1〜2段目(depth 0,1)は広い領域を素早く絞り込むため base(通常5)まで密に分割し、
// 3段目(depth 2)以降は上限を急に下げて下限3に張り付かせる(base=5 なら 5,5,3,3,…)。
// これにより深い段ほどセルが大きく・キー数が少なくなり、ラベルが読みやすく操作も
// 簡単になる。実際のグリッドは、この上限と MinLabelPx による上限の小さい方で決まる。
func gridCapAtDepth(depth, base int) int {
	if depth <= 1 {
		return base
	}
	c := base - 2*(depth-1) // 3段目で base-2、以降さらに小さく
	if c < 3 {
		return 3
	}
	if c > base {
		return base
	}
	return c
}

// clampInt は v を [lo, hi] の範囲に収める。
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// GenerateAnchors は displayArea と sourceArea を cols×rows のグリッドに分割し、
// 各セルにつき1つの Anchor を生成する。
//
// セルの幅・高さは切り捨て除算で算出し、余りのピクセルはすべて最後のセルに
// 加算することで、合計が常に領域全体と一致するようにする。
//
// 座標の対応付けは displayArea から sourceArea への線形(アフィン)変換であり、
// 呼び出し側が SourcePoint を整数の画面座標へ変換するまで丸めは行わない。
//
// 生成される Anchor は最大でも len(labels) 個である。グリッドのセル数が
// ラベル数を上回る場合は、先頭から len(labels) 個のセルのみを使用する。
func GenerateAnchors(cols, rows int, displayArea, sourceArea Rect, labels []Key) []Anchor {
	if cols <= 0 {
		cols = 1
	}
	if rows <= 0 {
		rows = 1
	}

	total := min(cols*rows, len(labels))

	// 表示座標を元座標へ対応付けるためのスケール係数。
	scaleX := sourceArea.W / displayArea.W
	scaleY := sourceArea.H / displayArea.H

	// 表示空間における基準セルの寸法(切り捨て除算)。
	baseCellW := displayArea.W / float64(cols)
	baseCellH := displayArea.H / float64(rows)

	anchors := make([]Anchor, 0, total)

	for idx := range total {
		col := idx % cols
		row := idx / cols

		// 表示空間におけるセルの原点。
		cellX := displayArea.X + float64(col)*baseCellW
		cellY := displayArea.Y + float64(row)*baseCellH

		// セルの寸法: 最終列・最終行が浮動小数点演算の余りを吸収する。
		cellW := baseCellW
		cellH := baseCellH
		if col == cols-1 {
			cellW = displayArea.X + displayArea.W - cellX
		}
		if row == rows-1 {
			cellH = displayArea.Y + displayArea.H - cellY
		}

		displayRect := Rect{X: cellX, Y: cellY, W: cellW, H: cellH}

		// 表示空間のセルを線形変換で元座標へ対応付ける。
		srcX := sourceArea.X + (cellX-displayArea.X)*scaleX
		srcY := sourceArea.Y + (cellY-displayArea.Y)*scaleY
		srcW := cellW * scaleX
		srcH := cellH * scaleY

		sourceAreaRect := Rect{X: srcX, Y: srcY, W: srcW, H: srcH}
		sourcePoint := sourceAreaRect.Center()

		anchors = append(anchors, Anchor{
			Label:       labels[idx],
			DisplayRect: displayRect,
			SourceArea:  sourceAreaRect,
			SourcePoint: sourcePoint,
		})
	}

	return anchors
}

// SourcePointPhysical は Anchor の SourcePoint を最も近い整数ピクセルに丸めて返す。
// 戻り値は Win32 の SetCursorPos へそのまま渡せる。
func SourcePointPhysical(a Anchor) (x, y int) {
	// 0.5 を加えてから整数変換することで math.Round 相当の丸めを行う。
	x = int(a.SourcePoint.X + 0.5)
	y = int(a.SourcePoint.Y + 0.5)
	return
}
