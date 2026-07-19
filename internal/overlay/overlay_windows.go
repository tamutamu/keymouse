//go:build windows

// Package overlay は、対象モニター全体を覆う枠なし・最前面のレイヤードウィンドウを
// 生成し、選択用のラベル(英字)を描画する。描画はすべて win32 パッケージの
// GDI ラッパー経由で行い、本パッケージから DLL を直接ロードすることはない。
package overlay

import (
	"fmt"

	"github.com/tamutamu/keymouse/internal/spatial"
	"github.com/tamutamu/keymouse/internal/win32"
)

const overlayClassName = "KeyMouseOverlay"
const cursorShieldClassName = "KeyMouseCursorShield"

// fontSizeMap は spatial.LabelSize を GDI のフォント高(ピクセル)へ対応付ける。
var fontSizeMap = map[spatial.LabelSize]int{
	spatial.LabelSmall:  14,
	spatial.LabelNormal: 16,
	spatial.LabelLarge:  18,
	spatial.LabelXLarge: 22,
}

// Overlay は選択ラベルを描画する全画面レイヤードウィンドウを表す。
// キー入力はフォーカスに依存しないキーボードフックで受け取るため、本ウィンドウは
// フォーカスを奪わず(WS_EX_NOACTIVATE)、キーメッセージも扱わない。
type Overlay struct {
	window       *win32.Window
	cursorShield *win32.Window
	originX      float64
	originY      float64
	anchors      []spatial.Anchor
	action       spatial.ClickAction
	depth        int
	loading      bool
	labelSize    spatial.LabelSize
	cursor       uintptr
}

// New は monRect で示すモニターを覆うオーバーレイウィンドウを生成する(表示はしない)。
func New(monRect win32.RECT, labelSize spatial.LabelSize) (*Overlay, error) {
	o := &Overlay{labelSize: labelSize, originX: float64(monRect.Left), originY: float64(monRect.Top)}
	if labelSize == 0 {
		o.labelSize = spatial.LabelNormal
	}

	cursor, err := win32.CreateTransparentCursor()
	if err != nil {
		return nil, fmt.Errorf("overlay transparent cursor: %w", err)
	}
	o.cursor = cursor
	shield, err := win32.CreateLayeredWindow(cursorShieldClassName, monRect, o.handleCursorShieldMessage)
	if err != nil {
		win32.DestroyCursor(o.cursor)
		return nil, fmt.Errorf("overlay cursor shield: %w", err)
	}
	// Alpha zero layered pixels are also mouse-transparent. Alpha one is
	// visually imperceptible but keeps the full monitor in hit testing.
	shield.SetLayeredAlpha(1)
	o.cursorShield = shield

	w, err := win32.CreateLayeredWindow(overlayClassName, monRect, o.handleMessage)
	if err != nil {
		o.cursorShield.DestroyWindow()
		win32.DestroyCursor(o.cursor)
		return nil, fmt.Errorf("overlay.New: %w", err)
	}
	o.window = w

	// Black is the transparent color. Only label pixels remain visible.
	o.window.SetLayeredColorKey(win32.RGB(0, 0, 0))
	return o, nil
}

// Show は指定したアンカー集合とクリック種別の表示でオーバーレイを表示する。
func (o *Overlay) Show(anchors []spatial.Anchor, depth int, action spatial.ClickAction) {
	o.loading = false
	o.anchors = anchors
	o.depth = depth
	o.action = action
	// フォーカスを奪わずに表示する(背後のメニュー等を閉じさせない)。
	o.cursorShield.ShowNoActivate()
	o.cursorShield.CaptureMouse()
	o.window.ShowNoActivate()
	win32.SetCursorHandle(o.cursor)
	o.window.Invalidate()
}

// ShowLoading displays a small non-activating progress indication while UIA
// discovery runs. It intentionally has no opaque panel.
func (o *Overlay) ShowLoading() {
	o.loading = true
	o.anchors = nil
	o.cursorShield.ShowNoActivate()
	o.cursorShield.CaptureMouse()
	o.window.ShowNoActivate()
	win32.SetCursorHandle(o.cursor)
	o.window.Invalidate()
}

// UpdateAnchors は現在のアンカー集合を差し替えて再描画する。
func (o *Overlay) UpdateAnchors(anchors []spatial.Anchor, depth int) {
	o.loading = false
	o.anchors = anchors
	o.depth = depth
	win32.SetCursorHandle(o.cursor)
	o.window.Invalidate()
}

// Hide temporarily hides the overlay without losing its selection state.
func (o *Overlay) Hide() {
	win32.ReleaseMouseCapture()
	o.window.Hide()
	o.cursorShield.Hide()
}

// Destroy はオーバーレイウィンドウを破棄して資源を解放する。
func (o *Overlay) Destroy() {
	win32.ReleaseMouseCapture()
	if o.window != nil {
		o.window.DestroyWindow()
		o.window = nil
	}
	if o.cursorShield != nil {
		o.cursorShield.DestroyWindow()
		o.cursorShield = nil
	}
	win32.DestroyCursor(o.cursor)
	o.cursor = 0
}

func (o *Overlay) handleCursorShieldMessage(hwnd uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
	switch msg {
	case win32.WM_SETCURSOR:
		win32.SetCursorHandle(o.cursor)
		return true, 1
	case win32.WM_PAINT:
		hdc, ps := win32.BeginPaint(hwnd)
		if hdc != 0 {
			r := win32.GetClientRect(hwnd)
			brush := win32.CreateSolidBrush(win32.RGB(0, 0, 0))
			if brush != 0 {
				win32.FillRect(hdc, &r, brush)
				win32.DeleteObject(brush)
			}
		}
		win32.EndPaint(hwnd, &ps)
		return true, 0
	case win32.WM_ERASEBKGND:
		return true, 1
	}
	return false, 0
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
	case win32.WM_SETCURSOR:
		// While the overlay owns hit testing, prevent DefWindowProc or the
		// underlying application from assigning a visible cursor.
		win32.SetCursorHandle(o.cursor)
		return true, 1

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

// paint draws labels on a color-keyed, fully transparent background.
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

	// The color-key background is completely transparent.
	bgBrush := win32.CreateSolidBrush(win32.RGB(0, 0, 0))
	if bgBrush != 0 {
		bgRect := win32.RECT{Left: 0, Top: 0, Right: int32(w), Bottom: int32(h)}
		win32.FillRect(memDC, &bgRect, bgBrush)
		win32.DeleteObject(bgBrush)
	}

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
		if o.loading {
			o.drawLoading(memDC, w, h)
		}

		win32.SelectObject(memDC, oldFont)
		win32.DeleteObject(font)
	}

	// メモリ DC から画面 DC へ一括転送する。
	win32.BitBlt(hdc, 0, 0, w, h, memDC, 0, 0)
}

func (o *Overlay) drawLoading(hdc uintptr, width, height int) {
	const label = "loading..."
	tw, th := win32.TextExtent(hdc, label)
	x, y := (width-tw)/2, (height-th)/2
	offsets := [][2]int{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, {0, -2}, {0, 2}, {-2, 0}, {2, 0}}
	win32.SetTextColor(hdc, win32.RGB(0x01, 0x01, 0x01))
	for _, offset := range offsets {
		win32.TextOut(hdc, x+offset[0], y+offset[1], label)
	}
	win32.SetTextColor(hdc, win32.RGB(0xFF, 0xFF, 0x00))
	win32.TextOut(hdc, x, y, label)
}

// stageFontSize は現在の段のセルサイズに収まるラベルフォント高(px)を返す。
// 設定上のラベルサイズを上限とし、最小セルの短辺に対して十分小さくなるよう縮小する。
// これにより最終段の小さなセルでもラベルが隣と重ならず、グリッド線と併せて判読できる。
func (o *Overlay) stageFontSize() int {
	configured := fontSizeMap[o.labelSize]
	if configured == 0 {
		configured = fontSizeMap[spatial.LabelNormal]
	}
	return configured
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
	if o.depth > 0 && o.depth < len(label) {
		label = label[o.depth:]
	}
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
	win32.SetTextColor(hdc, win32.RGB(0x01, 0x01, 0x01))
	for _, off := range offsets {
		win32.TextOut(hdc, x+off[0], y+off[1], label)
	}

	// 明るい本体テキストを描画する。
	win32.SetTextColor(hdc, win32.RGB(0xFF, 0xFF, 0x00)) // 黄色
	win32.TextOut(hdc, x, y, label)
}
