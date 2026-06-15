//go:build windows

package win32

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 本ファイルは設定ダイアログ等で使う「通常ウィンドウ」と標準子コントロール
// (ラベル/テキスト入力/チェックボックス/ボタン)生成のラッパーを提供する。
// これにより settings パッケージが Win32 のウィンドウクラス登録・WndProc・
// DLL ロードを自前で持つ必要がなくなる。

const (
	// 子コントロール用ウィンドウスタイル
	WS_CHILD   = 0x40000000
	WS_TABSTOP = 0x00010000
	WS_BORDER  = 0x00800000

	// 通常ウィンドウのスタイル (WS_OVERLAPPED|CAPTION|SYSMENU|MINIMIZEBOX|VISIBLE)
	wsAppWindow = 0x10CF0000

	// ボタンのスタイル
	BS_PUSHBUTTON    = 0x00000000
	BS_DEFPUSHBUTTON = 0x00000001
	BS_AUTOCHECKBOX  = 0x00000003

	// ボタンメッセージ
	BM_GETCHECK = 0x00F0
	BM_SETCHECK = 0x00F1

	// 子コントロールからの通知を受け取るウィンドウメッセージ
	WM_COMMAND = 0x0111

	// WM_COMMAND 通知コード
	BN_CLICKED = 0

	// COLOR_WINDOW+1 を背景ブラシとして使う
	colorWindowBrush = 6
)

var (
	procSendMessage        = user32.NewProc("SendMessageW")
	procGetDlgItemText     = user32.NewProc("GetDlgItemTextW")
	procGetDlgItemInt      = user32.NewProc("GetDlgItemInt")
	procSendDlgItemMessage = user32.NewProc("SendDlgItemMessageW")
)

// CreateAppWindow は枠付きの通常アプリケーションウィンドウを生成して返す。
// ウィンドウクラスは className 単位で一度だけ登録され、全メッセージは handler に
// 渡される(handler が処理しなかったメッセージは DefWindowProc に委譲される)。
func CreateAppWindow(className, title string, x, y, width, height int, handler MessageHandler) (*Window, error) {
	hInstance, _, _ := procGetModuleHandle.Call(0)
	if hInstance == 0 {
		return nil, fmt.Errorf("GetModuleHandle failed")
	}

	w := &Window{handler: handler}

	if !registeredClasses[className] {
		classNamePtr, _ := windows.UTF16PtrFromString(className)
		wc := WNDCLASSEX{
			Size:       uint32(unsafe.Sizeof(WNDCLASSEX{})),
			Style:      3, // CS_HREDRAW | CS_VREDRAW
			WndProc:    windows.NewCallback(globalWndProc),
			Instance:   windows.Handle(hInstance),
			Background: windows.Handle(colorWindowBrush),
			ClassName:  classNamePtr,
		}
		if r, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			return nil, fmt.Errorf("RegisterClassEx(%s): %w", className, err)
		}
		registeredClasses[className] = true
	}

	classNamePtr, _ := windows.UTF16PtrFromString(className)
	titlePtr, _ := windows.UTF16PtrFromString(title)
	hwnd, _, err := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		wsAppWindow,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowEx(%s): %w", className, err)
	}

	w.HWND = hwnd
	windowRegistry[hwnd] = w
	return w, nil
}

// createChild は指定クラスの子コントロールを生成する内部ヘルパー。
func createChild(parent uintptr, class, text string, style uint32, id, x, y, width, height int) uintptr {
	hInstance, _, _ := procGetModuleHandle.Call(0)
	clsPtr, _ := windows.UTF16PtrFromString(class)
	textPtr, _ := windows.UTF16PtrFromString(text)
	var exStyle uintptr
	if class == "EDIT" {
		exStyle = WS_BORDER
	}
	h, _, _ := procCreateWindowEx.Call(
		exStyle,
		uintptr(unsafe.Pointer(clsPtr)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		parent, uintptr(id), hInstance, 0,
	)
	return h
}

// CreateLabel は読み取り専用の静的ラベル(STATIC)を生成する。
func CreateLabel(parent uintptr, text string, x, y, width, height int) {
	createChild(parent, "STATIC", text, WS_CHILD|WS_VISIBLE, 0, x, y, width, height)
}

// CreateEdit は 1 行テキスト入力欄(EDIT)を生成する。id は後で値を読むための識別子。
func CreateEdit(parent uintptr, id int, text string, x, y, width, height int) {
	createChild(parent, "EDIT", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP, id, x, y, width, height)
}

// CreateCheckbox はチェックボックス(BUTTON+BS_AUTOCHECKBOX)を生成し、初期状態を設定する。
func CreateCheckbox(parent uintptr, id int, text string, x, y, width, height int, checked bool) {
	h := createChild(parent, "BUTTON", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX, id, x, y, width, height)
	if checked {
		procSendMessage.Call(h, BM_SETCHECK, 1, 0)
	}
}

// CreateButton は押しボタン(BUTTON)を生成する。defBtn が true なら既定ボタンにする。
func CreateButton(parent uintptr, id int, text string, x, y, width, height int, defBtn bool) {
	style := uint32(WS_CHILD | WS_VISIBLE | WS_TABSTOP | BS_PUSHBUTTON)
	if defBtn {
		style = WS_CHILD | WS_VISIBLE | WS_TABSTOP | BS_DEFPUSHBUTTON
	}
	createChild(parent, "BUTTON", text, style, id, x, y, width, height)
}

// GetDlgItemText は親ウィンドウ内の id を持つコントロールの文字列を返す。
func GetDlgItemText(hwnd uintptr, id int) string {
	buf := make([]uint16, 64)
	procGetDlgItemText.Call(hwnd, uintptr(id), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

// GetDlgItemInt は id を持つコントロールの内容を符号なし整数として返す。
func GetDlgItemInt(hwnd uintptr, id int) int {
	v, _, _ := procGetDlgItemInt.Call(hwnd, uintptr(id), 0, 0)
	return int(v)
}

// IsDlgButtonChecked は id を持つチェックボックスがチェック済みなら true を返す。
func IsDlgButtonChecked(hwnd uintptr, id int) bool {
	v, _, _ := procSendDlgItemMessage.Call(hwnd, uintptr(id), BM_GETCHECK, 0, 0)
	return v == 1
}

// ParseCommand は WM_COMMAND の wParam からコントロールIDと通知コードを取り出す。
func ParseCommand(wParam uintptr) (id int, notify int) {
	return int(wParam & 0xFFFF), int(wParam >> 16)
}
