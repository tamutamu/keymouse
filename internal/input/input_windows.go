//go:build windows

package input

import (
	"fmt"
	"log"

	"github.com/tamutamu/keymouse/internal/spatial"
	"github.com/tamutamu/keymouse/internal/win32"
)

// Manager はグローバルホットキーの登録を管理する(Win32 依存部分)。
type Manager struct {
	hwnd    uintptr
	hotkeys map[spatial.ClickAction]HotkeyConfig
}

// New は指定したウィンドウハンドル(スレッドレベルなら 0)に紐づく Manager を生成する。
// cfg が nil の場合は DefaultHotkeys を用いる。
func New(hwnd uintptr, cfg map[spatial.ClickAction]HotkeyConfig) *Manager {
	if cfg == nil {
		cfg = DefaultHotkeys()
	}
	return &Manager{hwnd: hwnd, hotkeys: cfg}
}

// Register は構成済みの全ホットキーを登録する。最初に失敗したものでエラーを返す。
func (m *Manager) Register() error {
	ids := idForAction()
	for action, cfg := range m.hotkeys {
		id := ids[action]
		if err := win32.RegisterHotKey(m.hwnd, id, cfg.Modifiers, cfg.VK); err != nil {
			return fmt.Errorf("input.Manager.Register: %w", err)
		}
		log.Printf("registered hotkey id=%d action=%v vk=0x%02X", id, action, cfg.VK)
	}
	return nil
}

// Unregister は全ホットキーの登録を解除する(ベストエフォート)。
func (m *Manager) Unregister() {
	for _, id := range idForAction() {
		_ = win32.UnregisterHotKey(m.hwnd, id)
	}
}
