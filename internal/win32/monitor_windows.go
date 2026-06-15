//go:build windows

package win32

import (
	"fmt"
	"unsafe"
)

// Monitor は物理モニター1台の矩形(物理ピクセル座標)を保持する。
type Monitor struct {
	Rect RECT
}

// MonitorFromPoint は点 (x,y) を含むモニターを返す。
func MonitorFromPoint(x, y int) (Monitor, error) {
	// x64 では POINT（int32 が2つ）を1つの 64 ビットレジスタにパックして渡す。
	pt := uintptr(uint32(int32(x))) | (uintptr(uint32(int32(y))) << 32)
	hmon, _, _ := procMonitorFromPoint.Call(pt, MONITOR_DEFAULTTONEAREST)
	if hmon == 0 {
		return Monitor{}, fmt.Errorf("MonitorFromPoint failed")
	}

	mi := MONITORINFO{Size: uint32(unsafe.Sizeof(MONITORINFO{}))}
	r, _, err := procGetMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&mi)))
	if r == 0 {
		return Monitor{}, fmt.Errorf("GetMonitorInfo: %w", err)
	}

	return Monitor{Rect: mi.Monitor}, nil
}
