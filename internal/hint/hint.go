// Package hint assigns short, prefix-friendly labels to normalized targets.
package hint

import (
	"sort"

	"github.com/tamutamu/keymouse/internal/target"
)

// Keys contains every unshifted letter available to Element Mode. H/J/K/L
// remain reserved for moving labels, while shifted G/R retain their existing
// mode-toggle and refresh meanings.
var Keys = []byte("asdfgtrewqzxcvbnyuiopm")

// MaxLabel3Items is the number of unique labels representable by the current
// three-key overlay (10 * 10 * 10). Element discovery must cap its candidates
// to this value until the renderer supports variable-width labels.
const MaxLabel3Items = 1000

type Item struct {
	Label  string
	Target target.Target
}

func Assign(targets []target.Target) []Item {
	if len(targets) == 0 {
		return nil
	}
	labels := shortestPrefixFreeLabels(len(targets))
	items := make([]Item, 0, len(targets))
	for i, t := range targets {
		items = append(items, Item{Label: labels[i], Target: t})
	}
	return items
}

// shortestPrefixFreeLabels keeps as many labels short as possible. Expanding
// one leaf into children preserves prefix-free selection while lengthening only
// the extra candidates, e.g. 11 targets become 9 one-key + 2 two-key labels.
func shortestPrefixFreeLabels(count int) []string {
	if count <= 0 {
		return nil
	}
	if count <= len(Keys) {
		labels := make([]string, count)
		for i := range labels {
			labels[i] = string(Keys[i])
		}
		return labels
	}
	leaves := make([]string, len(Keys))
	for i, key := range Keys {
		leaves[i] = string(key)
	}
	for len(leaves) < count {
		sort.SliceStable(leaves, func(i, j int) bool {
			if len(leaves[i]) != len(leaves[j]) {
				return len(leaves[i]) < len(leaves[j])
			}
			return leaves[i] < leaves[j]
		})
		// Expand the lexically last shortest leaf, retaining early spatial
		// targets as the easiest one-key labels.
		shortest := len(leaves[0])
		index := 0
		for i := range leaves {
			if len(leaves[i]) != shortest {
				break
			}
			index = i
		}
		prefix := leaves[index]
		leaves = append(leaves[:index], leaves[index+1:]...)
		children := count - len(leaves)
		if children > len(Keys) {
			children = len(Keys)
		}
		for i := 0; i < children; i++ {
			leaves = append(leaves, prefix+string(Keys[i]))
		}
	}
	sort.SliceStable(leaves, func(i, j int) bool {
		if len(leaves[i]) != len(leaves[j]) {
			return len(leaves[i]) < len(leaves[j])
		}
		return leaves[i] < leaves[j]
	})
	return leaves
}

func Filter(items []Item, prefix string) []Item {
	r := make([]Item, 0, len(items))
	for _, item := range items {
		if len(prefix) <= len(item.Label) && item.Label[:len(prefix)] == prefix {
			r = append(r, item)
		}
	}
	return r
}
