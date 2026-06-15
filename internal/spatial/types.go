// Package spatial は階層的なグリッド選択に関する座標計算やデータ構造を提供する。
// 画面領域をグリッドに分割し、各セルにラベルキーを割り当てることで、
// キー入力だけでマウスの目標位置を絞り込めるようにする。
package spatial

// Point は精度確保のため float64 で表現した2次元座標である。
type Point struct {
	X, Y float64
}

// Rect は軸に平行な矩形を表す。
type Rect struct {
	X, Y, W, H float64
}

// Contains は p が r の内部(辺上を含む)にあるかどうかを返す。
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X <= r.X+r.W &&
		p.Y >= r.Y && p.Y <= r.Y+r.H
}

// Center は矩形の中心点を返す。
func (r Rect) Center() Point {
	return Point{
		X: r.X + r.W/2,
		Y: r.Y + r.H/2,
	}
}

// Key は Win32 の仮想キーコードを表す。
type Key uint16

// ClickAction はマウスのクリック動作の種別を表す。
type ClickAction int

const (
	ClickLeft   ClickAction = 1
	ClickRight  ClickAction = 2
	ClickDouble ClickAction = 3
)

// Anchor は選択グリッド内のラベル付き領域を表す。
type Anchor struct {
	// Label はこのセルに表示される仮想キーコードである。
	Label Key
	// DisplayRect はオーバーレイ(画面)座標系におけるセルの矩形である。
	DisplayRect Rect
	// SourceArea はこれに対応する物理モニター座標系の矩形である。
	SourceArea Rect
	// SourcePoint は SourceArea の中心(マウスの目標位置)である。
	SourcePoint Point
}

// SelectionStage は階層的選択における1段階分の状態を保持する。
type SelectionStage struct {
	SearchArea Rect
	Anchors    []Anchor
}

// LabelSize はオーバーレイに表示されるキーラベルの大きさを制御する。
type LabelSize int

const (
	LabelSmall  LabelSize = 1
	LabelNormal LabelSize = 2
	LabelLarge  LabelSize = 3
	LabelXLarge LabelSize = 4
)
