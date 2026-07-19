// Package target defines the normalized UI targets shared by hinting and inspection.
package target

import "github.com/tamutamu/keymouse/internal/spatial"

type Source string

const (
	SourceUIAutomation Source = "uia"
	SourceGrid         Source = "grid"
)

type Role string

const (
	RoleUnknown   Role = "unknown"
	RoleButton    Role = "button"
	RoleCalendar  Role = "calendar"
	RoleCheckBox  Role = "checkbox"
	RoleComboBox  Role = "combobox"
	RoleEdit      Role = "edit"
	RoleHyperlink Role = "hyperlink"
	RoleListItem  Role = "listitem"
	RoleMenuItem  Role = "menuitem"
	RoleRadio     Role = "radio"
	RoleTabItem   Role = "tabitem"
	RoleTreeItem  Role = "treeitem"
)

type Action string

const (
	ActionInvoke Action = "invoke"
	ActionToggle Action = "toggle"
	ActionSelect Action = "select"
	ActionFocus  Action = "focus"
	ActionClick  Action = "click"
)

type Target struct {
	ID           string       `json:"id"`
	Name         string       `json:"name,omitempty"`
	Role         Role         `json:"role"`
	ControlType  string       `json:"controlType"`
	AutomationID string       `json:"automationId,omitempty"`
	ClassName    string       `json:"className,omitempty"`
	Bounds       spatial.Rect `json:"bounds"`
	Enabled      bool         `json:"enabled"`
	Offscreen    bool         `json:"offscreen"`
	Focusable    bool         `json:"keyboardFocusable"`
	Actions      []Action     `json:"actions,omitempty"`
	Source       Source       `json:"source"`
	Depth        int          `json:"depth,omitempty"`
}

func (t Target) Actionable() bool {
	return t.Enabled && !t.Offscreen && t.Bounds.W > 0 && t.Bounds.H > 0 && len(t.Actions) > 0
}
