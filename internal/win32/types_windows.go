//go:build windows

// Package win32 は Win32 API を隔離する低レベルラッパー層である。
// ウィンドウ生成・メッセージ処理・入力送信・モニター情報取得・描画など、
// OS 依存の処理をここに集約し、上位層が syscall や unsafe を直接扱わずに済むようにする。
package win32

import "golang.org/x/sys/windows"

// 本パッケージ全体で使用する Win32 定数。
const (
	// ウィンドウスタイル
	WS_POPUP          = 0x80000000
	WS_VISIBLE        = 0x10000000
	WS_EX_LAYERED     = 0x00080000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_NOACTIVATE  = 0x08000000
	WS_EX_TRANSPARENT = 0x00000020 // マウスヒットテストを透過(クリックスルー)

	// ウィンドウメッセージ
	WM_DESTROY    = 0x0002
	WM_PAINT      = 0x000F
	WM_HOTKEY     = 0x0312
	WM_KEYDOWN    = 0x0100
	WM_SYSKEYDOWN = 0x0104 // Alt 押下中のキーは WM_KEYDOWN ではなくこちらで届く
	WM_ERASEBKGND = 0x0014

	// 仮想キーコード
	VK_SHIFT  = 0x10
	VK_MENU   = 0x12 // Alt キー
	VK_LSHIFT = 0xA0
	VK_RSHIFT = 0xA1

	// keybd_event のフラグ
	KEYEVENTF_KEYUP = 0x0002

	// 低レベルキーボードフック
	WH_KEYBOARD_LL = 13

	// レイヤードウィンドウの属性
	LWA_ALPHA = 0x00000002

	// GDI 定数
	SRCCOPY     = 0x00CC0020
	TRANSPARENT = 1

	// GetKeyState の押下判定マスク
	KEY_PRESSED = 0x8000

	// SendInput のマウスフラグ
	MOUSEEVENTF_LEFTDOWN  = 0x0002
	MOUSEEVENTF_LEFTUP    = 0x0004
	MOUSEEVENTF_RIGHTDOWN = 0x0008
	MOUSEEVENTF_RIGHTUP   = 0x0010

	// モニター定数
	MONITOR_DEFAULTTONEAREST = 2

	// ShowWindow 定数
	SW_SHOW   = 5
	SW_SHOWNA = 8 // アクティブ化せずに表示(フォーカスを奪わない)
	SW_HIDE   = 0
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterHotKey             = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey           = user32.NewProc("UnregisterHotKey")
	procGetMessage                 = user32.NewProc("GetMessageW")
	procDispatchMessage            = user32.NewProc("DispatchMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procCreateWindowEx             = user32.NewProc("CreateWindowExW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procDefWindowProc              = user32.NewProc("DefWindowProcW")
	procRegisterClassEx            = user32.NewProc("RegisterClassExW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procGetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
	procKeybdEvent                 = user32.NewProc("keybd_event")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procSetWindowsHookEx           = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx        = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx             = user32.NewProc("CallNextHookEx")
	procSetCursorPos               = user32.NewProc("SetCursorPos")
	procSendInput                  = user32.NewProc("SendInput")
	procGetDoubleClickTime         = user32.NewProc("GetDoubleClickTime")
	procMonitorFromPoint           = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfo             = user32.NewProc("GetMonitorInfoW")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procFillRect                   = user32.NewProc("FillRect")
	procGetCursorPos               = user32.NewProc("GetCursorPos")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procTextOut                = gdi32.NewProc("TextOutW")
	procGetTextExtentPoint32   = gdi32.NewProc("GetTextExtentPoint32W")

	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

// RECT は Win32 の RECT 構造体に対応する。
type RECT struct {
	Left, Top, Right, Bottom int32
}

// POINT は Win32 の POINT 構造体に対応する。
type POINT struct {
	X, Y int32
}

// SIZE は Win32 の SIZE 構造体に対応する(幅・高さ)。
type SIZE struct {
	CX, CY int32
}

// MSG は Win32 の MSG 構造体に対応する。
type MSG struct {
	Hwnd    windows.HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

// PAINTSTRUCT は Win32 の PAINTSTRUCT 構造体に対応する。
type PAINTSTRUCT struct {
	Hdc         windows.Handle
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

// WNDCLASSEX は Win32 の WNDCLASSEX 構造体に対応する。
type WNDCLASSEX struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

// MONITORINFO は Win32 の MONITORINFO 構造体に対応する。
type MONITORINFO struct {
	Size     uint32
	Monitor  RECT
	WorkArea RECT
	Flags    uint32
}

// inputRecord はマウス入力用の Win32 INPUT 構造体に対応する（x64 では 40 バイト）。
// バイトレイアウト: type(4) + pad(4) + dx(4) + dy(4) + mouseData(4) + flags(4) + time(4) + pad(4) + extraInfo(8)
type inputRecord [40]byte

// newMouseInput はマウスイベント用の inputRecord を構築する。
func newMouseInput(flags uint32) inputRecord {
	var inp inputRecord
	// type = INPUT_MOUSE (0) はバイト 0〜3。ゼロのままでよい。
	// flags はオフセット 20。
	inp[20] = byte(flags)
	inp[21] = byte(flags >> 8)
	inp[22] = byte(flags >> 16)
	inp[23] = byte(flags >> 24)
	return inp
}

// sizeofInputRecord は x64 における Win32 INPUT 構造体のサイズ（40 バイト）。
const sizeofInputRecord = 40
