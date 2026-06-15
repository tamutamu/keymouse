//go:build windows

package win32

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CreateMessageWindow は描画を行わず、グローバルメッセージ（WM_HOTKEY やトレイの
// コールバックなど）を受信するための非表示なメッセージ専用ウィンドウを作成する。
func CreateMessageWindow(className string, handler MessageHandler) (*Window, error) {
	hInstance, _, _ := procGetModuleHandle.Call(0)
	if hInstance == 0 {
		return nil, fmt.Errorf("GetModuleHandle failed")
	}

	w := &Window{handler: handler}

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

	// HWND_MESSAGE (-3) を親に指定すると、表示領域を持たないメッセージ専用ウィンドウになる。
	const HWND_MESSAGE = ^uintptr(2)
	classNamePtr2, _ := windows.UTF16PtrFromString(className)
	hwnd, _, err := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr2)),
		0,
		0,
		0, 0, 0, 0,
		HWND_MESSAGE, 0,
		hInstance, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowEx(message): %w", err)
	}

	w.HWND = hwnd
	windowRegistry[hwnd] = w
	return w, nil
}
