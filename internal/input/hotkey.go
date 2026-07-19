// Package input はグローバルホットキーの登録と、押下キーの判定を担う。
// 純粋ロジック(本ファイル)は OS 非依存でテスト可能とし、実際の Win32 への
// 登録処理は input_windows.go の Manager に隔離している。
package input

import "github.com/tamutamu/keymouse/internal/spatial"

// RegisterHotKey に渡すホットキーID。
const (
	HotkeyIDSingle     = 1
	HotkeyIDRight      = 2
	HotkeyIDDouble     = 3
	HotkeyIDContinuous = 4
)

// ホットキー修飾キー。Win32 の MOD_* と同値だが、本パッケージを OS 非依存に
// 保つため独自に定義する(win32 への依存を避けてテスト可能にする)。
const (
	ModAlt      = 0x0001
	ModShift    = 0x0004
	ModNoRepeat = 0x4000
)

// HotkeyConfig は 1 つのホットキーの仮想キーと修飾キーを保持する。
// settings の永続化(config.json)でもこの型をそのまま用いるため JSON タグを付ける。
type HotkeyConfig struct {
	VK        uint32 `json:"vk"`
	Modifiers uint32 `json:"modifiers"`
}

type Binding struct {
	ID     int
	Config HotkeyConfig
}

func DefaultBindings() []Binding {
	return []Binding{
		{ID: HotkeyIDSingle, Config: HotkeyConfig{VK: 0x4B, Modifiers: ModAlt | ModNoRepeat}},     // Alt+K
		{ID: HotkeyIDContinuous, Config: HotkeyConfig{VK: 0x49, Modifiers: ModAlt | ModNoRepeat}}, // Alt+I
		{ID: HotkeyIDRight, Config: HotkeyConfig{VK: 0x52, Modifiers: ModAlt | ModNoRepeat}},      // Alt+R
		{ID: HotkeyIDDouble, Config: HotkeyConfig{VK: 0x44, Modifiers: ModAlt | ModNoRepeat}},     // Alt+D
	}
}

// ActionForHotkeyID は WM_HOTKEY の wParam(ホットキーID)を ClickAction に対応付ける。
// 自分のホットキーでなければ (0, false) を返す。
func IsKnownHotkeyID(id uintptr) bool {
	switch int(id) {
	case HotkeyIDSingle, HotkeyIDContinuous, HotkeyIDRight, HotkeyIDDouble:
		return true
	}
	return false
}

// IsLabelKey reports whether vk is one of A/S/D/F/G/T.
func IsLabelKey(vk uintptr) bool {
	return spatial.IsGridKey(spatial.Key(vk))
}
