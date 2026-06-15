package input

import (
	"testing"

	"github.com/tamutamu/keymouse/internal/spatial"
)

func TestActionForHotkeyID(t *testing.T) {
	cases := []struct {
		id     uintptr
		want   spatial.ClickAction
		wantOK bool
	}{
		{HotkeyIDLeft, spatial.ClickLeft, true},
		{HotkeyIDRight, spatial.ClickRight, true},
		{HotkeyIDDouble, spatial.ClickDouble, true},
		{999, 0, false},
	}
	for _, c := range cases {
		got, ok := ActionForHotkeyID(c.id)
		if got != c.want || ok != c.wantOK {
			t.Errorf("ActionForHotkeyID(%d) = (%v,%v), want (%v,%v)", c.id, got, ok, c.want, c.wantOK)
		}
	}
}

func TestIsLabelKey(t *testing.T) {
	if !IsLabelKey(0x41) { // A
		t.Error("0x41 (A) should be a label key")
	}
	if !IsLabelKey(0x59) { // Y
		t.Error("0x59 (Y) should be a label key")
	}
	if IsLabelKey(0x5A) { // Z は対象外(ラベルは A〜Y の25個)
		t.Error("0x5A (Z) should not be a label key")
	}
	if IsLabelKey(0x1B) { // Esc
		t.Error("Esc should not be a label key")
	}
}

func TestDefaultHotkeysCoversAllActions(t *testing.T) {
	hk := DefaultHotkeys()
	for _, action := range []spatial.ClickAction{spatial.ClickLeft, spatial.ClickRight, spatial.ClickDouble} {
		cfg, ok := hk[action]
		if !ok {
			t.Errorf("default hotkeys missing action %v", action)
			continue
		}
		if cfg.Modifiers&ModAlt == 0 {
			t.Errorf("action %v should use Alt modifier", action)
		}
	}
}
