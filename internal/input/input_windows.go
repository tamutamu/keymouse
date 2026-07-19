//go:build windows

package input

import (
	"fmt"
	"github.com/tamutamu/keymouse/internal/win32"
	"log"
)

// Manager はグローバルホットキーの登録を管理する(Win32 依存部分)。
type Manager struct {
	hwnd    uintptr
	hotkeys []Binding
}

// New は指定したウィンドウハンドル(スレッドレベルなら 0)に紐づく Manager を生成する。
// cfg が nil の場合は DefaultHotkeys を用いる。
func New(hwnd uintptr, cfg []Binding) *Manager {
	if cfg == nil {
		cfg = DefaultBindings()
	}
	return &Manager{hwnd: hwnd, hotkeys: cfg}
}

// Register は構成済みの全ホットキーを登録する。最初に失敗したものでエラーを返す。
func (m *Manager) Register() error {
	for _, binding := range m.hotkeys {
		if err := win32.RegisterHotKey(m.hwnd, binding.ID, binding.Config.Modifiers, binding.Config.VK); err != nil {
			return fmt.Errorf("input.Manager.Register: %w", err)
		}
		log.Printf("registered hotkey id=%d vk=0x%02X", binding.ID, binding.Config.VK)
	}
	return nil
}

// Unregister は全ホットキーの登録を解除する(ベストエフォート)。
func (m *Manager) Unregister() {
	for _, binding := range m.hotkeys {
		_ = win32.UnregisterHotKey(m.hwnd, binding.ID)
	}
}
