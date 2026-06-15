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
	// キーボード入力(WM_KEYDOWN)を受け取れるようフォーカスを移す。
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

	case win32.WM_KEYDOWN:
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

	// 各アンカーのラベルを描画する。
	fontSize := fontSizeMap[o.labelSize]
	font := win32.CreateFontBold(fontSize)
	if font != 0 {
		oldFont := win32.SelectObject(memDC, font)
		win32.SetBkMode(memDC, win32.TRANSPARENT)

		for _, a := range o.anchors {
			o.drawAnchorLabel(memDC, a)
		}

		win32.SelectObject(memDC, oldFont)
		win32.DeleteObject(font)
	}

	// 画面下部にステータスバーを描画する。
	o.drawStatusBar(memDC, w, h)

	// メモリ DC から画面 DC へ一括転送する。
	win32.BitBlt(hdc, 0, 0, w, h, memDC, 0, 0)
}

// drawAnchorLabel は可読性のため縁取り(ハロー)付きで 1 つのラベルを描画する。
func (o *Overlay) drawAnchorLabel(hdc uintptr, a spatial.Anchor) {
	label := spatial.KeyToChar(a.Label)
	if label == "" {
		return
	}

	// オーバーレイ座標系でのセル中心。
	cx := int(a.DisplayRect.X + a.DisplayRect.W/2)
	cy := int(a.DisplayRect.Y + a.DisplayRect.H/2)

	fontSize := fontSizeMap[o.labelSize]
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
