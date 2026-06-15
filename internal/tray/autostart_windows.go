//go:build windows

package tray

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	appValue = "KeyMouse"
)

// SetAutoStart はレジストリの Run キーを用いて Windows 起動時の自動起動を有効/無効にする。
func SetAutoStart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("autostart: open registry: %w", err)
	}
	defer k.Close()

	if enable {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("autostart: get exe path: %w", err)
		}
		if err := k.SetStringValue(appValue, `"`+exe+`"`); err != nil {
			return fmt.Errorf("autostart: set registry value: %w", err)
		}
	} else {
		if err := k.DeleteValue(appValue); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("autostart: delete registry value: %w", err)
		}
	}
	return nil
}
