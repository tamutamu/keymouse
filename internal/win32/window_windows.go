//go:build windows

package win32

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// MessageHandler はウィンドウメッセージを処理する関数である。
// 戻り値は (処理したか bool, 結果 uintptr)。
type MessageHandler func(hwnd uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr)

// Window は Win32 の HWND と、それに対応するメッセージハンドラをまとめて保持する。
type Window struct {
	HWND    uintptr
	handler MessageHandler
}

// registeredClasses はこのプロセスで登録済みのクラス名を記録する。
var registeredClasses = map[string]bool{}

// CreateLayeredWindow は rect（物理ピクセル座標）で指定したモニター上に、
// フルスクリーンで枠なし・最前面のレイヤードウィンドウを作成する。
// ハンドラは DefWindowProc より前にすべてのメッセージを受け取る。
func CreateLayeredWindow(className string, rect RECT, handler MessageHandler) (*Window, error) {
	hInstance, _, _ := procGetModuleHandle.Call(0)
	if hInstance == 0 {
		return nil, fmt.Errorf("GetModuleHandle failed")
	}

	w := &Window{handler: handler}

	// ウィンドウクラスはクラス名ごとに一度だけ登録する。
	if !registeredClasses[className] {
		classNamePtr, _ := windows.UTF16PtrFromString(className)

		wc := WNDCLASSEX{
			Size:      uint32(unsafe.Sizeof(WNDCLASSEX{})),
			WndProc:   windows.NewCallback(globalWndProc),
			Instance:  windows.Handle(hInstance),
			ClassName: classNamePtr,
		}
		r, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
		if r == 0 {
			return nil, fmt.Errorf("RegisterClassEx(%s): %w", className, err)
		}
		registeredClasses[className] = true
	}

	classNamePtr2, _ := windows.UTF16PtrFromString(className)

	// WS_EX_NOACTIVATE: フォーカスを奪わない(開いていたメニュー等を閉じさせない)。
	// WS_EX_TRANSPARENT: マウスのヒットテストを透過(クリックスルー)する。これにより
	// オーバーレイ表示中もカーソル下の判定は背後のウィンドウのままとなり、ホバーで
	// 展開するメニュー等が「マウスが離れた」と判定されて閉じてしまうのを防ぐ。
	// キー入力はフォーカス非依存の低レベルキーボードフック(hook_windows.go)で受け取る。
	hwnd, _, err := procCreateWindowEx.Call(
		uintptr(WS_EX_LAYERED|WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE|WS_EX_TRANSPARENT),
		uintptr(unsafe.Pointer(classNamePtr2)),
		0,
		uintptr(WS_POPUP),
		uintptr(rect.Left), uintptr(rect.Top),
		uintptr(rect.Right-rect.Left), uintptr(rect.Bottom-rect.Top),
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowEx(%s): %w", className, err)
	}

	w.HWND = hwnd
	windowRegistry[hwnd] = w
	return w, nil
}

// DestroyWindow はウィンドウを破棄する。
func (w *Window) DestroyWindow() {
	if w.HWND != 0 {
		delete(windowRegistry, w.HWND)
		procDestroyWindow.Call(w.HWND)
		w.HWND = 0
	}
}

// Show はウィンドウを表示する。
func (w *Window) Show() {
	procShowWindow.Call(w.HWND, SW_SHOW)
}

// ShowNoActivate はウィンドウをアクティブ化せずに表示する(フォーカスを奪わない)。
func (w *Window) ShowNoActivate() {
	procShowWindow.Call(w.HWND, SW_SHOWNA)
}

// Hide はウィンドウを非表示にする。
func (w *Window) Hide() {
	procShowWindow.Call(w.HWND, SW_HIDE)
}

// Invalidate は再描画を要求する。
func (w *Window) Invalidate() {
	procInvalidateRect.Call(w.HWND, 0, 1)
}

// SetLayeredAlpha はウィンドウ全体のアルファ値を設定する（0=透明、255=不透明）。
func (w *Window) SetLayeredAlpha(alpha byte) {
	procSetLayeredWindowAttributes.Call(w.HWND, 0, uintptr(alpha), LWA_ALPHA)
}

// windowRegistry は WndProc のディスパッチ用に HWND と *Window を対応付ける。
var windowRegistry = map[uintptr]*Window{}

// globalWndProc は本パッケージ経由で登録されたすべてのウィンドウに対して
// 呼び出される Win32 ウィンドウプロシージャである。
func globalWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if w, ok := windowRegistry[hwnd]; ok && w.handler != nil {
		if handled, result := w.handler(hwnd, uint32(msg), wParam, lParam); handled {
			return result
		}
	}
	r, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return r
}

// RunMessageLoop は WM_QUIT を受け取るまで Win32 メッセージポンプを実行する。
// PostQuitMessage が呼ばれると処理を抜ける。
func RunMessageLoop() {
	var msg MSG
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 || r == ^uintptr(0) { // 0 = WM_QUIT、-1 = エラー
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// PostQuitMessage はメッセージキューに WM_QUIT を投稿する。
func PostQuitMessage(exitCode int) {
	procPostQuitMessage.Call(uintptr(exitCode))
}

// BeginPaint は描画を開始し、DC と PAINTSTRUCT を返す。
func BeginPaint(hwnd uintptr) (uintptr, PAINTSTRUCT) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	return hdc, ps
}

// EndPaint は描画を終了する。
func EndPaint(hwnd uintptr, ps *PAINTSTRUCT) {
	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(ps)))
}

// GetClientRect はクライアント領域の矩形を返す。
func GetClientRect(hwnd uintptr) RECT {
	var r RECT
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

// ForegroundWindowRect returns the visible bounds of the active top-level window.
func ForegroundWindowRect() (RECT, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return RECT{}, fmt.Errorf("GetForegroundWindow returned 0")
	}
	var r RECT
	ok, _, err := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ok == 0 {
		return RECT{}, fmt.Errorf("GetWindowRect: %w", err)
	}
	return r, nil
}
