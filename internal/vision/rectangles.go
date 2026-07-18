// Package vision contains the small, dependency-free rectangle detector used
// by the second selection stage.  Its input is a screen capture; replacing it
// with a GoCV detector later keeps the app/session API unchanged.
package vision

import (
	"image"
	"sort"
)

// Rectangles finds visually bounded, button-sized regions.  It uses an edge
// map and connected components, then rejects text-sized and page-sized noise.
// The returned rectangles are local to the supplied image.
func Rectangles(bgra []byte, width, height int) []image.Rectangle {
	if width < 8 || height < 8 || len(bgra) < width*height*4 {
		return nil
	}
	edge := make([]bool, width*height)
	gray := func(x, y int) int {
		p := (y*width + x) * 4
		return (int(bgra[p])*11 + int(bgra[p+1])*59 + int(bgra[p+2])*30) / 100
	}
	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			g := gray(x, y)
			if abs(g-gray(x-1, y))+abs(g-gray(x+1, y))+abs(g-gray(x, y-1))+abs(g-gray(x, y+1)) > 170 {
				edge[y*width+x] = true
			}
		}
	}
	seen := make([]bool, len(edge))
	result := make([]image.Rectangle, 0, 12)
	for start, on := range edge {
		if !on || seen[start] {
			continue
		}
		q := []int{start}
		seen[start] = true
		minX, maxX := start%width, start%width
		minY, maxY := start/width, start/width
		n := 0
		for len(q) > 0 {
			p := q[len(q)-1]
			q = q[:len(q)-1]
			n++
			x, y := p%width, p/width
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= width || ny >= height {
						continue
					}
					ni := ny*width + nx
					if edge[ni] && !seen[ni] {
						seen[ni] = true
						q = append(q, ni)
					}
				}
			}
		}
		w, h := maxX-minX+1, maxY-minY+1
		if n < 24 || w < 24 || h < 16 || w > width*9/10 || h > height*9/10 {
			continue
		}
		if w*h > width*height*3/4 {
			continue
		}
		result = append(result, image.Rect(minX, minY, maxX+1, maxY+1))
	}
	sort.Slice(result, func(i, j int) bool {
		ai := result[i].Dx() * result[i].Dy()
		aj := result[j].Dx() * result[j].Dy()
		if ai == aj {
			return result[i].Min.Y < result[j].Min.Y
		}
		return ai > aj
	})
	return suppressContained(result)
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func suppressContained(in []image.Rectangle) []image.Rectangle {
	out := make([]image.Rectangle, 0, len(in))
	for _, r := range in {
		duplicate := false
		for _, kept := range out {
			if r.Min.X >= kept.Min.X-3 && r.Min.Y >= kept.Min.Y-3 && r.Max.X <= kept.Max.X+3 && r.Max.Y <= kept.Max.Y+3 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, r)
		}
		if len(out) == 21 {
			break
		}
	}
	return out
}
