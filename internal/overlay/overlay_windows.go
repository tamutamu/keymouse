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
	spatial.LabelSmall:  14,
	spatial.LabelNormal: 20,
	spatial.LabelLarge:  28,
	spatial.LabelXLarge: 40,
}

// overlayAlpha はオーバーレイウィンドウ全体の不透明度(0=透明〜255=不透明)。
// 背後の実画面が透けて見える程度に抑え、暗い背景塗りと合わせて画面を薄暗くする
// (方式B: ズームせず実画面の上にラベルを重ねる)。
const overlayAlpha = 140

// Overlay は選択ラベルを描画する全画面レイヤードウィンドウを表す。
type Overlay struct {
	window    *win32.Window
	anchors   []spatial.Anchor
	action    spatial.ClickAction
	labelSize spatial.LabelSize

	// OnKeyDown はオーバーレイが WM_KEYDOWN を受け取った際に呼ばれる。
	OnKeyDown func(vk uintptr)
}

// New は monRect で示すモニターを覆うオーバーレイウィンドウを生成する(表示はしない)。
func New(monRect win32.RECT, labelSize spatial.LabelSize) (*Overlay, error) {
	o := &Overlay{labelSize: labelSize}
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
	o.window.Show()
	o.window.Invalidate()
	// キーボード入力(WM_KEYDOWN)を受け取れるよう、前面化してフォーカスを移す。
	// ホットキー押下直後はフォアグラウンド権が付与されているため前面化が成功する。
	win32.SetForegroundWindow(o.window.HWND)
	win32.SetFocus(o.window.HWND)
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

	case win32.WM_KEYDOWN, win32.WM_SYSKEYDOWN:
		// WM_SYSKEYDOWN は Alt 押下中のキー(Alt+h/j/k/; のグリッド移動)で届く。
		// どちらも同じハンドラへ渡し、処理済みとして既定処理(システムメニュー等)を抑止する。
		if o.OnKeyDown != nil {
			o.OnKeyDown(wParam)
		}
		return true, 0
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
	font := win32.CreateFontBold(fontSize)
	if font != 0 {
		oldFont := win32.SelectObject(memDC, font)
		win32.SetBkMode(memDC, win32.TRANSPARENT)

		for _, a := range o.anchors {
			o.drawAnchorLabel(memDC, a, fontSize)
		}

		win32.SelectObject(memDC, oldFont)
		win32.DeleteObject(font)
	}

	// 画面下部にステータスバーを描画する。
	o.drawStatusBar(memDC, w, h)

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
	minSide := math.MaxFloat64
	for _, a := range o.anchors {
		side := math.Min(a.DisplayRect.W, a.DisplayRect.H)
		if side < minSide {
			minSide = side
		}
	}

	// セル短辺と同じ高さをフォント高とし、上限は設定値、下限は判読可能な10pxとする。
	// 文字の実描画高はフォント高の約7割なので、等倍でも枠やグリッド線に触れない。
	fit := int(minSide * 1.0)
	if fit >= configured {
		return configured
	}
	if fit < 10 {
		fit = 10
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

	const t = 1 // 線の太さ(px)
	for _, a := range o.anchors {
		r := a.DisplayRect
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
// fontSize は呼び出し側で算出した現在の段のフォント高(stageFontSize)。
func (o *Overlay) drawAnchorLabel(hdc uintptr, a spatial.Anchor, fontSize int) {
	label := spatial.KeyToChar(a.Label)
	if label == "" {
		return
	}

	// オーバーレイ座標系でのセル中心。
	cx := int(a.DisplayRect.X + a.DisplayRect.W/2)
	cy := int(a.DisplayRect.Y + a.DisplayRect.H/2)

	halfSize := fontSize / 2

	// 暗いハロー/影を周囲 8 方向にオフセット描画する(明暗どちらの背景でも読めるように)。
	offsets := [][2]int{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, {0, -2}, {0, 2}, {-2, 0}, {2, 0}}
	win32.SetTextColor(hdc, win32.RGB(0x00, 0x00, 0x00))
	for _, off := range offsets {
		win32.TextOut(hdc, cx-halfSize+off[0], cy-halfSize+off[1], label)
	}

	// 明るい本体テキストを描画する。
	win32.SetTextColor(hdc, win32.RGB(0xFF, 0xFF, 0x00)) // 黄色
	win32.TextOut(hdc, cx-halfSize, cy-halfSize, label)
}

// drawStatusBar は画面下部に操作ヒント(クリック種別・操作キー)を描画する。
func (o *Overlay) drawStatusBar(hdc uintptr, w, h int) {
	var statusText string
	switch o.action {
	case spatial.ClickLeft:
		statusText = "Left Click  |  Shift+key = click now  |  Backspace = back  |  Esc = cancel"
	case spatial.ClickRight:
		statusText = "Right Click  |  Shift+key = click now  |  Backspace = back  |  Esc = cancel"
	case spatial.ClickDouble:
		statusText = "Double Click  |  Shift+key = click now  |  Backspace = back  |  Esc = cancel"
	default:
		return
	}

	barH := 24
	barRect := win32.RECT{
		Left:   0,
		Top:    int32(h - barH),
		Right:  int32(w),
		Bottom: int32(h),
	}
	barBrush := win32.CreateSolidBrush(win32.RGB(0x0D, 0x0D, 0x0D))
	if barBrush != 0 {
		win32.FillRect(hdc, &barRect, barBrush)
		win32.DeleteObject(barBrush)
	}

	smallFont := win32.CreateFontBold(12)
	if smallFont != 0 {
		oldFont := win32.SelectObject(hdc, smallFont)
		win32.SetTextColor(hdc, win32.RGB(0xCC, 0xCC, 0xCC))
		win32.TextOut(hdc, 8, h-barH+5, statusText)
		win32.SelectObject(hdc, oldFont)
		win32.DeleteObject(smallFont)
	}
}
