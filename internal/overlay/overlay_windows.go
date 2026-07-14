//go:build windows

// Package overlay は、対象モニター全体を覆う枠なし・最前面のレイヤードウィンドウを
// 生成し、選択用のラベル(英字)を描画する。描画はすべて win32 パッケージの
// GDI ラッパー経由で行い、本パッケージから DLL を直接ロードすることはない。
package overlay

import (
	"fmt"
	"math"

	"github.com/tamutamu/keymouse/internal/spatial"
	"github.com/tamutamu/keymouse/internal/win32"
)

const overlayClassName = "KeyMouseOverlay"

// fontSizeMap は spatial.LabelSize を GDI のフォント高(ピクセル)へ対応付ける。
var fontSizeMap = map[spatial.LabelSize]int{
	spatial.LabelSmall:  10,
	spatial.LabelNormal: 11,
	spatial.LabelLarge:  14,
	spatial.LabelXLarge: 18,
}

// overlayAlpha はオーバーレイウィンドウ全体の不透明度(0=透明〜255=不透明)。
// 背後の実画面が透けて見える程度に抑え、暗い背景塗りと合わせて画面を薄暗くする
// (方式B: ズームせず実画面の上にラベルを重ねる)。
const overlayAlpha = 140

// Overlay は選択ラベルを描画する全画面レイヤードウィンドウを表す。
// キー入力はフォーカスに依存しないキーボードフックで受け取るため、本ウィンドウは
// フォーカスを奪わず(WS_EX_NOACTIVATE)、キーメッセージも扱わない。
type Overlay struct {
	window    *win32.Window
	originX   float64
	originY   float64
	anchors   []spatial.Anchor
	action    spatial.ClickAction
	labelSize spatial.LabelSize
}

// New は monRect で示すモニターを覆うオーバーレイウィンドウを生成する(表示はしない)。
func New(monRect win32.RECT, labelSize spatial.LabelSize) (*Overlay, error) {
	o := &Overlay{labelSize: labelSize, originX: float64(monRect.Left), originY: float64(monRect.Top)}
	if labelSize == 0 {
		o.labelSize = spatial.LabelNormal
	}

	w, err := win32.CreateLayeredWindow(overlayClassName, monRect, o.handleMessage)
	if err != nil {
		return nil, fmt.Errorf("overlay.New: %w", err)
	}
	o.window = w

	// ウィンドウ全体を半透明にして実画面を透過させる。
	o.window.SetLayeredAlpha(overlayAlpha)
	return o, nil
}

// Show は指定したアンカー集合とクリック種別の表示でオーバーレイを表示する。
func (o *Overlay) Show(anchors []spatial.Anchor, action spatial.ClickAction) {
	o.anchors = anchors
	o.action = action
	// フォーカスを奪わずに表示する(背後のメニュー等を閉じさせない)。
	o.window.ShowNoActivate()
	o.window.Invalidate()
}

// UpdateAnchors は現在のアンカー集合を差し替えて再描画する。
func (o *Overlay) UpdateAnchors(anchors []spatial.Anchor) {
	o.anchors = anchors
	o.window.Invalidate()
}

// Hide はオーバーレイを非表示にし、アンカー情報を破棄する。
func (o *Overlay) Hide() {
	o.anchors = nil
	o.window.Hide()
}

// Destroy はオーバーレイウィンドウを破棄して資源を解放する。
func (o *Overlay) Destroy() {
	if o.window != nil {
		o.window.DestroyWindow()
		o.window = nil
	}
}

// HWND は内部のウィンドウハンドルを返す。
func (o *Overlay) HWND() uintptr {
	if o.window == nil {
		return 0
	}
	return o.window.HWND
}

// handleMessage はオーバーレイウィンドウのウィンドウプロシージャである。
func (o *Overlay) handleMessage(hwnd uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
	switch msg {
	case win32.WM_PAINT:
		hdc, ps := win32.BeginPaint(hwnd)
		if hdc != 0 {
			o.paint(hdc)
		}
		win32.EndPaint(hwnd, &ps)
		return true, 0

	case win32.WM_ERASEBKGND:
		// 背景消去は paint 側で行うため、ここでは何もせず処理済みを返す(ちらつき防止)。
		return true, 1
	}
	return false, 0
}

// paint は半透明の背景と全アンカーのラベルを描画する。
// ちらつきを防ぐためメモリ DC への二重バッファ描画を行う。
func (o *Overlay) paint(hdc uintptr) {
	cr := win32.GetClientRect(o.window.HWND)
	w := int(cr.Right - cr.Left)
	h := int(cr.Bottom - cr.Top)

	// 二重バッファ用のメモリ DC を生成する。
	memDC := win32.CreateCompatibleDC(hdc)
	if memDC == 0 {
		return
	}
	defer win32.DeleteDC(memDC)

	bmp := win32.CreateCompatibleBitmap(hdc, w, h)
	if bmp == 0 {
		return
	}
	defer win32.DeleteObject(bmp)

	oldBmp := win32.SelectObject(memDC, bmp)
	defer win32.SelectObject(memDC, oldBmp)

	// 背景を暗い半透明オーバーレイ色で塗りつぶす。
	bgBrush := win32.CreateSolidBrush(win32.RGB(0x1A, 0x1A, 0x2E)) // 濃紺
	if bgBrush != 0 {
		bgRect := win32.RECT{Left: 0, Top: 0, Right: int32(w), Bottom: int32(h)}
		win32.FillRect(memDC, &bgRect, bgBrush)
		win32.DeleteObject(bgBrush)
	}

	// セルの境界を示すグリッド線を描画する(現在の段の全セルの枠線)。
	o.drawGridLines(memDC)

	// 各アンカーのラベルを描画する。フォント高はセルに収まるよう自動調整する
	// (段が深くセルが小さいほど小さくなり、隣のラベルと重ならない)。
	fontSize := o.stageFontSize()
	font := win32.CreateFontSemiBold(fontSize)
	if font != 0 {
		oldFont := win32.SelectObject(memDC, font)
		win32.SetBkMode(memDC, win32.TRANSPARENT)

		for _, a := range o.anchors {
			o.drawAnchorLabel(memDC, a)
		}

		win32.SelectObject(memDC, oldFont)
		win32.DeleteObject(font)
	}

	// メモリ DC から画面 DC へ一括転送する。
	win32.BitBlt(hdc, 0, 0, w, h, memDC, 0, 0)
}

// stageFontSize は現在の段のセルサイズに収まるラベルフォント高(px)を返す。
// 設定上のラベルサイズを上限とし、最小セルの短辺に対して十分小さくなるよう縮小する。
// これにより最終段の小さなセルでもラベルが隣と重ならず、グリッド線と併せて判読できる。
func (o *Overlay) stageFontSize() int {
	configured := fontSizeMap[o.labelSize]
	if configured == 0 {
		configured = fontSizeMap[spatial.LabelNormal]
	}
	if len(o.anchors) == 0 {
		return configured
	}

	// 全セルの中で最も短い辺を求める(最終行・最終列は余りを吸収して大きめなので、
	// 通常セルの辺長が下限になる)。
	minSide := 1e100
	for _, a := range o.anchors {
		side := math.Min(a.DisplayRect.W, a.DisplayRect.H)
		if side < minSide {
			minSide = side
		}
	}

	// 文字サイズをさらに小さくするため 0.7 倍する。下限は 8px。
	fit := int(minSide * 0.7)
	if fit >= configured {
		return configured
	}
	if fit < 8 {
		fit = 8
	}
	return fit
}

// drawGridLines は現在の段の全セルの枠線(グリッド線)を描画する。
func (o *Overlay) drawGridLines(hdc uintptr) {
	brush := win32.CreateSolidBrush(win32.RGB(0x5A, 0x5A, 0x8C)) // 薄い青紫の線
	if brush == 0 {
		return
	}
	defer win32.DeleteObject(brush)

	t := int32(1)
	for _, a := range o.anchors {
		r := a.DisplayRect
		r.X -= o.originX
		r.Y -= o.originY
		x0, y0 := int32(r.X), int32(r.Y)
		x1, y1 := int32(r.X+r.W), int32(r.Y+r.H)

		top := win32.RECT{Left: x0, Top: y0, Right: x1, Bottom: y0 + t}
		bottom := win32.RECT{Left: x0, Top: y1 - t, Right: x1, Bottom: y1}
		left := win32.RECT{Left: x0, Top: y0, Right: x0 + t, Bottom: y1}
		right := win32.RECT{Left: x1 - t, Top: y0, Right: x1, Bottom: y1}

		win32.FillRect(hdc, &top, brush)
		win32.FillRect(hdc, &bottom, brush)
		win32.FillRect(hdc, &left, brush)
		win32.FillRect(hdc, &right, brush)
	}
}

// drawAnchorLabel は可読性のため縁取り(ハロー)付きで 1 つのラベルを描画する。
// クリック先はセル中心なので、文字の実寸を測ってセル中心へ正確にセンタリングする
// (固定の fontSize/2 では1文字の実幅とずれ、クリックが視覚より右にずれてしまう)。
func (o *Overlay) drawAnchorLabel(hdc uintptr, a spatial.Anchor) {
	label := spatial.Label3ToStr(a.Label)
	if label == "" {
		return
	}

	// オーバーレイ座標系でのセル中心。
	cx := int(a.DisplayRect.X - o.originX + a.DisplayRect.W/2)
	cy := int(a.DisplayRect.Y - o.originY + a.DisplayRect.H/2)

	// 文字の実寸から左上原点を求め、セル中心に合わせる。
	tw, th := win32.TextExtent(hdc, label)
	x := cx - tw/2
	y := cy - th/2

	// 暗いハロー/影を周囲 8 方向にオフセット描画する(明暗どちらの背景でも読めるように)。
	offsets := [][2]int{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, {0, -2}, {0, 2}, {-2, 0}, {2, 0}}
	win32.SetTextColor(hdc, win32.RGB(0x00, 0x00, 0x00))
	for _, off := range offsets {
		win32.TextOut(hdc, x+off[0], y+off[1], label)
	}

	// 明るい本体テキストを描画する。
	win32.SetTextColor(hdc, win32.RGB(0xFF, 0xFF, 0x00)) // 黄色
	win32.TextOut(hdc, x, y, label)
}
