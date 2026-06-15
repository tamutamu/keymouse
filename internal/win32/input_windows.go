//go:build windows

package win32

import (
	"fmt"
	"time"
	"unsafe"
)

// SetCursorPos はカーソルを指定したスクリーン座標へ移動する。
func SetCursorPos(x, y int) error {
	r, _, err := procSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos(%d,%d): %w", x, y, err)
	}
	return nil
}

// sendMouseInput は SendInput を使って1件のマウス入力イベントを送信する。
func sendMouseInput(flags uint32) error {
	inp := newMouseInput(flags)
	r, _, err := procSendInput.Call(
		1,
		uintptr(unsafe.Pointer(&inp)),
		sizeofInputRecord,
	)
	if r == 0 {
		return fmt.Errorf("SendInput: %w", err)
	}
	return nil
}

// ClickLeft は左マウスボタンの押下と解放を送信する。
func ClickLeft() error {
	if err := sendMouseInput(MOUSEEVENTF_LEFTDOWN); err != nil {
		return err
	}
	return sendMouseInput(MOUSEEVENTF_LEFTUP)
}

// ClickRight は右マウスボタンの押下と解放を送信する。
func ClickRight() error {
	if err := sendMouseInput(MOUSEEVENTF_RIGHTDOWN); err != nil {
		return err
	}
	return sendMouseInput(MOUSEEVENTF_RIGHTUP)
}

// ClickDouble は GetDoubleClickTime の範囲内で左クリックを2回送信する。
func ClickDouble() error {
	dct, _, _ := procGetDoubleClickTime.Call()
	interval := time.Duration(dct) * time.Millisecond / 3

	if err := sendMouseInput(MOUSEEVENTF_LEFTDOWN); err != nil {
		return err
	}
	if err := sendMouseInput(MOUSEEVENTF_LEFTUP); err != nil {
		return err
	}
	time.Sleep(interval)
	if err := sendMouseInput(MOUSEEVENTF_LEFTDOWN); err != nil {
		return err
	}
	return sendMouseInput(MOUSEEVENTF_LEFTUP)
}

// IsShiftPressed は左右いずれかの Shift キーが現在押されていれば true を返す。
func IsShiftPressed() bool {
	state, _, _ := procGetKeyState.Call(VK_SHIFT)
	return state&KEY_PRESSED != 0
}

// GetCursorPos は現在のカーソル位置をスクリーン座標で返す。
func GetCursorPos() (x, y int, err error) {
	var pt POINT
	r, _, e := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return 0, 0, fmt.Errorf("GetCursorPos: %w", e)
	}
	return int(pt.X), int(pt.Y), nil
}
