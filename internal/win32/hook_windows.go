//go:build windows

package win32

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 本ファイルは低レベルキーボードフック(WH_KEYBOARD_LL)を提供する。
// オーバーレイはフォーカスを奪わない(背後のメニュー等を閉じさせない)ため、
// WM_KEYDOWN ではなくこのグローバルフックでキー入力を受け取る。

// KBDLLHOOKSTRUCT は WH_KEYBOARD_LL フックの lParam が指す構造体。
type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

var (
	kbHook        uintptr                         // 設置中のフックハンドル(0=未設置)
	kbCallback    func(vk uint32, down bool) bool // 非修飾キーのコールバック
	hookAltDown   bool                            // Alt 押下状態(フックで追跡)
	hookShiftDown bool                            // Shift 押下状態(フックで追跡)

	// kbHookProcPtr はフックプロシージャの C コールバック。生成は一度きり。
	kbHookProcPtr = windows.NewCallback(keyboardHookProc)
)

// modifierKind は修飾キーの種別を判定する。
func modifierKind(vk uint32) (isAlt, isShift, isModifier bool) {
	switch vk {
	case VK_MENU, 0xA4, 0xA5: // Alt / 左Alt / 右Alt
		return true, false, true
	case VK_SHIFT, VK_LSHIFT, VK_RSHIFT:
		return false, true, true
	case 0x11, 0xA2, 0xA3: // Ctrl / 左Ctrl / 右Ctrl
		return false, false, true
	}
	return false, false, false
}

// keyboardHookProc は全キー入力に対して呼ばれるフックプロシージャ。
// 全キー(修飾キーを含む)をコールバックへ渡す。
// ただし飲み込む(背後アプリへ渡さない)のは非修飾キーのみとし、修飾キー(Shift/Ctrl/Alt)は
// コールバックの戻り値に関わらず必ず通過させる(システムの修飾状態を壊さないため)。
// lParam(KBDLLHOOKSTRUCT へのポインタ)は OS が指す有効領域なので *KBDLLHOOKSTRUCT で
// 直接受け取り、uintptr からのポインタ復元(go vet が警告する変換)を避ける。
func keyboardHookProc(nCode uintptr, wParam uintptr, kb *KBDLLHOOKSTRUCT) uintptr {
	if int32(nCode) >= 0 && kb != nil {
		down := wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN
		isAlt, isShift, isModifier := modifierKind(kb.VkCode)
		switch {
		case isAlt:
			hookAltDown = down
		case isShift:
			hookShiftDown = down
		}
		if kbCallback != nil {
			swallow := kbCallback(kb.VkCode, down)
			if swallow && !isModifier {
				return 1 // 飲み込む(背後アプリへ渡さない)
			}
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, nCode, wParam, uintptr(unsafe.Pointer(kb)))
	return r
}

// SetKeyboardHook は低レベルキーボードフックを設置する。cb には押下/離上(down)と
// 仮想キーが渡され、true を返したキーは他アプリへ届かない。設置時点の Alt/Shift の
// 物理状態を初期値として取り込む(フック設置前に押された Alt 等を取りこぼさない)。
func SetKeyboardHook(cb func(vk uint32, down bool) bool) error {
	hInst, _, _ := procGetModuleHandle.Call(0)
	h, _, err := procSetWindowsHookEx.Call(WH_KEYBOARD_LL, kbHookProcPtr, hInst, 0)
	if h == 0 {
		return fmt.Errorf("SetWindowsHookEx(WH_KEYBOARD_LL): %w", err)
	}
	kbCallback = cb
	kbHook = h
	hookAltDown = asyncKeyDown(VK_MENU)
	hookShiftDown = asyncKeyDown(VK_SHIFT)
	return nil
}

// RemoveKeyboardHook はキーボードフックを解除する。
func RemoveKeyboardHook() {
	if kbHook != 0 {
		procUnhookWindowsHookEx.Call(kbHook)
		kbHook = 0
	}
	kbCallback = nil
	hookAltDown = false
	hookShiftDown = false
}

// asyncKeyDown は GetAsyncKeyState で指定キーが現在押されていれば true を返す。
func asyncKeyDown(vk uintptr) bool {
	state, _, _ := procGetAsyncKeyState.Call(vk)
	return state&KEY_PRESSED != 0
}
