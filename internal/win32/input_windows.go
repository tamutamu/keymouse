//go:build windows

package win32

import (
	"fmt"
	"time"
	"unsafe"
)

// HideCursor decrements Windows' cursor display counter until the cursor is
// hidden and returns the number of decrements. The caller must pass that exact
// value to RestoreCursor; this preserves the pre-existing counter state.
func HideCursor() (int, uintptr) {
	calls := 0
	for calls < 32 {
		result, _, _ := procShowCursor.Call(0)
		calls++
		if int32(result) < 0 {
			break
		}
	}
	previous, _, _ := procSetCursor.Call(0)
	return calls, previous
}

// RestoreCursor reverses a previous HideCursor call exactly.
func RestoreCursor(calls int, previous uintptr) {
	for i := 0; i < calls; i++ {
		procShowCursor.Call(1)
	}
	if previous != 0 {
		procSetCursor.Call(previous)
	}
}

// ClearCursor immediately removes a cursor image that an underlying window
// may have set again while the selection overlay remains active.
func ClearCursor() { procSetCursor.Call(0) }

// CreateTransparentCursor creates a monochrome cursor whose AND mask keeps
// every destination pixel and whose XOR mask changes none of them. The cursor
// is therefore visually transparent even when Windows considers it shown.
func CreateTransparentCursor() (uintptr, error) {
	andMask := [128]byte{}
	for i := range andMask {
		andMask[i] = 0xFF
	}
	xorMask := [128]byte{}
	handle, _, err := procCreateCursor.Call(
		0, 0, 0, 32, 32,
		uintptr(unsafe.Pointer(&andMask[0])),
		uintptr(unsafe.Pointer(&xorMask[0])),
	)
	if handle == 0 {
		return 0, fmt.Errorf("CreateCursor: %w", err)
	}
	return handle, nil
}

func SetCursorHandle(cursor uintptr) { procSetCursor.Call(cursor) }

func DestroyCursor(cursor uintptr) {
	if cursor != 0 {
		procDestroyCursor.Call(cursor)
	}
}

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

// IsShiftPressed は Shift キーが押されていれば true を返す。
// キーボードフック設置中はフックが追跡する状態を、未設置時は物理状態を用いる
// (フックがキーを飲み込むため、フック中は GetAsyncKeyState が当てにならない)。
func IsShiftPressed() bool {
	if kbHook != 0 {
		return hookShiftDown
	}
	return asyncKeyDown(VK_SHIFT)
}

// IsAltPressed は Alt キーが押されていれば true を返す(判定方針は IsShiftPressed と同じ)。
func IsAltPressed() bool {
	if kbHook != 0 {
		return hookAltDown
	}
	return asyncKeyDown(VK_MENU)
}

// ReleaseShift は左右の Shift キーのキーアップを合成送信し、押下状態を解除する。
// 「Shift+ラベルで即クリック」する際、物理的に押されたままの Shift がクリックに
// 修飾として漏れ、対象アプリが Shift+クリックとして受け取ってしまうのを防ぐ。
// 既に離されているキーへのキーアップは無害なので、無条件に呼んでよい。
func ReleaseShift() {
	procKeybdEvent.Call(VK_LSHIFT, 0, KEYEVENTF_KEYUP, 0)
	procKeybdEvent.Call(VK_RSHIFT, 0, KEYEVENTF_KEYUP, 0)
	procKeybdEvent.Call(VK_SHIFT, 0, KEYEVENTF_KEYUP, 0)
}

// InjectEscape はシステムに対して Escape キーの押下と解放を送信する。
// タイムアウト時の自動キャンセル等で用いる。
func InjectEscape() {
	procKeybdEvent.Call(0x1B, 0, 0, 0) // VK_ESCAPE
	procKeybdEvent.Call(0x1B, 0, 2, 0) // KEYEVENTF_KEYUP
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
