//go:build windows

package win32

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 本ファイルは GDI 描画プリミティブの型付きラッパーを提供する。
// これにより overlay などの上位パッケージが gdi32.dll / user32.dll を
// 再ロードすることなく、win32 パッケージ経由で描画できる。

// RGB は RGB 各成分を Win32 の COLORREF (0x00BBGGRR) に変換する。
func RGB(r, g, b byte) uintptr {
	return uintptr(r) | (uintptr(g) << 8) | (uintptr(b) << 16)
}

// CreateCompatibleDC は hdc と互換なメモリデバイスコンテキストを生成する。
// 二重バッファ描画に用いる。不要になったら DeleteDC で解放すること。
func CreateCompatibleDC(hdc uintptr) uintptr {
	dc, _, _ := procCreateCompatibleDC.Call(hdc)
	return dc
}

// DeleteDC はデバイスコンテキストを破棄する。
func DeleteDC(dc uintptr) {
	procDeleteDC.Call(dc)
}

// CreateCompatibleBitmap は hdc と互換な width×height のビットマップを生成する。
func CreateCompatibleBitmap(hdc uintptr, width, height int) uintptr {
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(width), uintptr(height))
	return bmp
}

// DeleteObject はブラシ・フォント・ビットマップなどの GDI オブジェクトを破棄する。
func DeleteObject(obj uintptr) {
	procDeleteObject.Call(obj)
}

// SelectObject は GDI オブジェクトをデバイスコンテキストに選択し、
// 直前に選択されていたオブジェクトのハンドルを返す。
func SelectObject(dc, obj uintptr) uintptr {
	old, _, _ := procSelectObject.Call(dc, obj)
	return old
}

// BitBlt は src デバイスコンテキストの矩形を dst へ単純コピー (SRCCOPY) する。
func BitBlt(dst uintptr, dstX, dstY, width, height int, src uintptr, srcX, srcY int) {
	procBitBlt.Call(
		dst, uintptr(dstX), uintptr(dstY), uintptr(width), uintptr(height),
		src, uintptr(srcX), uintptr(srcY), SRCCOPY,
	)
}

// CaptureBGRA copies an absolute desktop rectangle into a top-down BGRA image.
// It is deliberately independent of the overlay, which is not visible yet
// when the detector is invoked.
func CaptureBGRA(left, top, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid capture size")
	}
	screen, _, err := procGetDC.Call(0)
	if screen == 0 {
		return nil, err
	}
	defer procReleaseDC.Call(0, screen)
	mem := CreateCompatibleDC(screen)
	if mem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer DeleteDC(mem)
	bmp := CreateCompatibleBitmap(screen, width, height)
	if bmp == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer DeleteObject(bmp)
	old := SelectObject(mem, bmp)
	defer SelectObject(mem, old)
	BitBlt(mem, 0, 0, width, height, screen, left, top)
	bmi := BITMAPINFO{Header: BITMAPINFOHEADER{Size: uint32(unsafe.Sizeof(BITMAPINFOHEADER{})), Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32}}
	pixels := make([]byte, width*height*4)
	r, _, callErr := procGetDIBits.Call(mem, bmp, 0, uintptr(height), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&bmi)), 0)
	if r == 0 {
		return nil, callErr
	}
	return pixels, nil
}

// CreateSolidBrush は指定色 (COLORREF) の単色ブラシを生成する。
func CreateSolidBrush(color uintptr) uintptr {
	brush, _, _ := procCreateSolidBrush.Call(color)
	return brush
}

// FillRect は rect 領域を brush で塗りつぶす。
func FillRect(dc uintptr, rect *RECT, brush uintptr) {
	procFillRect.Call(dc, uintptr(unsafe.Pointer(rect)), brush)
}

// SetBkMode は背景描画モードを設定する (テキスト描画では win32.TRANSPARENT を用いる)。
func SetBkMode(dc uintptr, mode int) {
	procSetBkMode.Call(dc, uintptr(mode))
}

// SetTextColor はテキスト描画色 (COLORREF) を設定する。
func SetTextColor(dc, color uintptr) {
	procSetTextColor.Call(dc, color)
}

// TextOut は (x,y) を左上原点として文字列を描画する。
func TextOut(dc uintptr, x, y int, text string) {
	utf16, err := windows.UTF16FromString(text)
	if err != nil || len(utf16) == 0 {
		return
	}
	// 末尾の NUL を除いた文字数を渡す。
	procTextOut.Call(dc, uintptr(x), uintptr(y),
		uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)-1))
}

// TextExtent は現在 dc に選択されているフォントで text を描画したときの
// 幅・高さ(ピクセル)を返す。ラベルをセル中心へ正確にセンタリングするために用いる。
func TextExtent(dc uintptr, text string) (w, h int) {
	utf16, err := windows.UTF16FromString(text)
	if err != nil || len(utf16) == 0 {
		return 0, 0
	}
	var sz SIZE
	procGetTextExtentPoint32.Call(dc,
		uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)-1),
		uintptr(unsafe.Pointer(&sz)))
	return int(sz.CX), int(sz.CY)
}

// CreateFontSemiBold は指定ピクセル高のやや太字の ClearType フォント (Segoe UI) を生成する。
// 不要になったら DeleteObject で解放すること。
func CreateFontSemiBold(height int) uintptr {
	faceName, _ := windows.UTF16FromString("Segoe UI")
	font, _, _ := procCreateFontW.Call(
		uintptr(height), // 高さ
		0,               // 幅 (自動)
		0,               // エスケープメント
		0,               // 方向
		600,             // 太さ FW_SEMIBOLD
		0,               // イタリック
		0,               // 下線
		0,               // 打ち消し線
		0,               // 文字セット ANSI_CHARSET
		0,               // 出力精度
		0,               // クリップ精度
		5,               // 描画品質 CLEARTYPE_QUALITY
		0,               // ピッチとファミリ
		uintptr(unsafe.Pointer(&faceName[0])),
	)
	return font
}
