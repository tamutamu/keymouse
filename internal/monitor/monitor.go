// Package monitor はディスプレイモニターの情報を扱う型を提供する。
// 各モニターの位置とサイズ(物理ピクセル座標)を保持する。
package monitor

// Info はディスプレイモニターの矩形(物理ピクセル座標)を保持する。
type Info struct {
	Left, Top, Right, Bottom int
}

// Width はモニターの幅を物理ピクセル単位で返す。
func (m Info) Width() int { return m.Right - m.Left }

// Height はモニターの高さを物理ピクセル単位で返す。
func (m Info) Height() int { return m.Bottom - m.Top }
