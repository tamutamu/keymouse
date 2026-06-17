// Package input はグローバルホットキーの登録と、押下キーの判定を担う。
// 純粋ロジック(本ファイル)は OS 非依存でテスト可能とし、実際の Win32 への
// 登録処理は input_windows.go の Manager に隔離している。
package input

import "github.com/tamutamu/keymouse/internal/spatial"

// RegisterHotKey に渡すホットキーID。
const (
	HotkeyIDLeft   = 1
	HotkeyIDRight  = 2
	HotkeyIDDouble = 3
)

// ホットキー修飾キー。Win32 の MOD_* と同値だが、本パッケージを OS 非依存に
// 保つため独自に定義する(win32 への依存を避けてテスト可能にする)。
const (
	ModAlt      = 0x0001
	ModNoRepeat = 0x4000
)

// HotkeyConfig は 1 つのホットキーの仮想キーと修飾キーを保持する。
// settings の永続化(config.json)でもこの型をそのまま用いるため JSON タグを付ける。
type HotkeyConfig struct {
	VK        uint32 `json:"vk"`
	Modifiers uint32 `json:"modifiers"`
}

// DefaultHotkeys は既定のホットキー構成(Alt+N=左 / Alt+R=右 / Alt+D=ダブル)を返す。
// 左クリックは Alt+N。Alt+L を空けることで、オーバーレイ表示中の右方向パンを
// vim 流の Alt+l に割り当てられる(Alt+h/j/k/l)。
func DefaultHotkeys() map[spatial.ClickAction]HotkeyConfig {
	return map[spatial.ClickAction]HotkeyConfig{
		spatial.ClickLeft:   {VK: 0x4E, Modifiers: ModAlt | ModNoRepeat}, // Alt+N
		spatial.ClickRight:  {VK: 0x52, Modifiers: ModAlt | ModNoRepeat}, // Alt+R
		spatial.ClickDouble: {VK: 0x44, Modifiers: ModAlt | ModNoRepeat}, // Alt+D
	}
}

// idForAction はクリック種別からホットキーIDへの対応を返す。
func idForAction() map[spatial.ClickAction]int {
	return map[spatial.ClickAction]int{
		spatial.ClickLeft:   HotkeyIDLeft,
		spatial.ClickRight:  HotkeyIDRight,
		spatial.ClickDouble: HotkeyIDDouble,
	}
}

// ActionForHotkeyID は WM_HOTKEY の wParam(ホットキーID)を ClickAction に対応付ける。
// 自分のホットキーでなければ (0, false) を返す。
func ActionForHotkeyID(id uintptr) (spatial.ClickAction, bool) {
	switch int(id) {
	case HotkeyIDLeft:
		return spatial.ClickLeft, true
	case HotkeyIDRight:
		return spatial.ClickRight, true
	case HotkeyIDDouble:
		return spatial.ClickDouble, true
	}
	return 0, false
}

// IsLabelKey は vk が A〜Y のラベルキー(仮想キーコード 0x41〜0x59)なら true を返す。
func IsLabelKey(vk uintptr) bool {
	return vk >= 0x41 && vk <= 0x59
}
