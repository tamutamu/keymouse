//go:build windows

package win32

import "fmt"

// RegisterHotKey は指定したID・修飾キー・仮想キーコードでグローバルホットキーを登録する。
// スレッド単位のホットキー（メッセージループ用ウィンドウで使用）の場合、hwnd は 0 を指定する。
func RegisterHotKey(hwnd uintptr, id int, modifiers uint32, vk uint32) error {
	r, _, err := procRegisterHotKey.Call(hwnd, uintptr(id), uintptr(modifiers), uintptr(vk))
	if r == 0 {
		return fmt.Errorf("RegisterHotKey id=%d: %w", id, err)
	}
	return nil
}

// UnregisterHotKey は登録済みのホットキーの登録を解除する。
func UnregisterHotKey(hwnd uintptr, id int) error {
	r, _, err := procUnregisterHotKey.Call(hwnd, uintptr(id))
	if r == 0 {
		return fmt.Errorf("UnregisterHotKey id=%d: %w", id, err)
	}
	return nil
}
