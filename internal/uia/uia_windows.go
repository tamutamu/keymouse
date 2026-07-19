//go:build windows

// Package uia provides a deliberately small Windows UI Automation client.
// COM pointers remain private; callers receive normalized target.Target values.
package uia

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/tamutamu/keymouse/internal/spatial"
	"github.com/tamutamu/keymouse/internal/target"
	"github.com/tamutamu/keymouse/internal/win32"
	"golang.org/x/sys/windows"
)

const (
	coinitApartmentThreaded = 2
	clsctxInprocServer      = 1
	sOK                     = 0

	propertyName                 = 30005
	propertyAutomationID         = 30011
	propertyClassName            = 30012
	propertyControlType          = 30003
	propertyBoundingRectangle    = 30001
	propertyIsEnabled            = 30010
	propertyIsKeyboardFocusable  = 30009
	propertyIsOffscreen          = 30022
	propertyIsInvokeAvailable    = 30031
	propertyIsSelectionAvailable = 30079
	propertyIsToggleAvailable    = 30086
	propertyLegacyDefaultAction  = 30100

	patternInvoke        = 10000
	patternSelectionItem = 10010
	patternToggle        = 10015

	vtEmpty = 0
	vtI4    = 3
	vtR8    = 5
	vtBSTR  = 8
	vtBool  = 11
	vtArray = 0x2000

	treeScopeElement     = 1
	treeScopeDescendants = 4
)

var (
	ole32                     = windows.NewLazySystemDLL("ole32.dll")
	oleaut32                  = windows.NewLazySystemDLL("oleaut32.dll")
	procCoInitializeEx        = ole32.NewProc("CoInitializeEx")
	procCoUninitialize        = ole32.NewProc("CoUninitialize")
	procCoCreateInstance      = ole32.NewProc("CoCreateInstance")
	procVariantClear          = oleaut32.NewProc("VariantClear")
	procSafeArrayAccessData   = oleaut32.NewProc("SafeArrayAccessData")
	procSafeArrayUnaccessData = oleaut32.NewProc("SafeArrayUnaccessData")
	procSafeArrayGetLBound    = oleaut32.NewProc("SafeArrayGetLBound")
	procSafeArrayGetUBound    = oleaut32.NewProc("SafeArrayGetUBound")
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
)

var (
	clsidCUIAutomation = windows.GUID{Data1: 0xFF48DBA4, Data2: 0x60EF, Data3: 0x4201, Data4: [8]byte{0xAA, 0x87, 0x54, 0x10, 0x3E, 0xEF, 0x59, 0x4E}}
	iidIUIAutomation   = windows.GUID{Data1: 0x30CBE57D, Data2: 0xD9D0, Data3: 0x452A, Data4: [8]byte{0xAB, 0x13, 0x7A, 0xC5, 0xAC, 0x48, 0x25, 0xEE}}
)

type iunknown struct{ vtbl *uintptr }
type variant struct {
	VT  uint16
	_   [3]uint16
	Val uint64
	_2  uint64
}

type Client struct {
	automation  *iunknown
	initialized bool
}

type targetsResult struct {
	targets []target.Target
	err     error
}

// Only one UIA traversal may touch third-party providers at a time. Timed-out
// COM calls cannot be forcefully cancelled; bounding concurrency prevents
// abandoned traversals from piling up and making every later activation worse.
var discoverySlot = make(chan struct{}, 1)
var executionSlot = make(chan struct{}, 1)

var (
	ErrDiscoveryTimeout = errors.New("UI Automation discovery timed out")
	ErrDiscoveryBusy    = errors.New("previous UI Automation discovery is still running")
	ErrActionTimeout    = errors.New("UI Automation action timed out; outcome is unknown")
	ErrActionBusy       = errors.New("previous UI Automation action is still running")
)

// DiscoverForeground runs UIA on its own COM apartment. Some third-party UIA
// providers block indefinitely; the timeout keeps the hotkey path responsive.
func DiscoverForeground(timeout time.Duration) ([]target.Target, error) {
	return discoverForeground(timeout, true)
}

// DiscoverForegroundTree is the exhaustive variant used by Inspector.
func DiscoverForegroundTree(timeout time.Duration) ([]target.Target, error) {
	return discoverForeground(timeout, false)
}

func discoverForeground(timeout time.Duration, actionableOnly bool) ([]target.Target, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case discoverySlot <- struct{}{}:
	case <-timer.C:
		return nil, ErrDiscoveryBusy
	}
	ch := make(chan targetsResult, 1)
	go func() {
		defer func() { <-discoverySlot }()
		client, err := New()
		if err != nil {
			ch <- targetsResult{err: err}
			return
		}
		defer client.Close()
		var targets []target.Target
		if actionableOnly {
			targets, err = client.ForegroundActionableTargets()
		} else {
			targets, err = client.ForegroundTargets()
		}
		ch <- targetsResult{targets: targets, err: err}
	}()
	select {
	case result := <-ch:
		return result.targets, result.err
	case <-timer.C:
		return nil, fmt.Errorf("%w after %s", ErrDiscoveryTimeout, timeout)
	}
}

func (c *Client) ForegroundActionableTargets() ([]target.Target, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil, fmt.Errorf("GetForegroundWindow returned 0")
	}
	return c.actionableTargets(hwnd)
}

// ActionableTargets discovers interactive candidates from an explicitly
// captured window, avoiding foreground-window races during diagnostics.
func (c *Client) ActionableTargets(hwnd uintptr) ([]target.Target, error) {
	if hwnd == 0 {
		return nil, fmt.Errorf("window handle is 0")
	}
	return c.actionableTargets(hwnd)
}

func (c *Client) actionableTargets(hwnd uintptr) ([]target.Target, error) {
	windowRect, err := win32.WindowRect(hwnd)
	if err != nil {
		return nil, err
	}
	viewport := spatial.Rect{
		X: float64(windowRect.Left), Y: float64(windowRect.Top),
		W: float64(windowRect.Right - windowRect.Left), H: float64(windowRect.Bottom - windowRect.Top),
	}
	var root *iunknown
	hr := call(c.automation, 6, hwnd, uintptr(unsafe.Pointer(&root)))
	if failed(hr) || root == nil {
		return nil, hresult("IUIAutomation.ElementFromHandle", hr)
	}
	defer release(root)
	var walker *iunknown
	hr = call(c.automation, 14, uintptr(unsafe.Pointer(&walker)))
	if failed(hr) || walker == nil {
		return nil, hresult("IUIAutomation.ControlViewWalker", hr)
	}
	defer release(walker)
	cache, err := c.interactiveCacheRequest()
	if err != nil {
		return nil, err
	}
	defer release(cache)
	if targets, err := c.findVisibleTargets(root, cache, viewport); err == nil {
		return targets, nil
	}
	// Some older/nonstandard providers reject FindAllBuildCache conditions.
	// Retain the cached walker as a compatibility path, not the primary path.
	var out []target.Target
	state := actionableWalkState{}
	c.walkActionable(walker, cache, root, 0, viewport, &out, &state)
	return out, nil
}

func (c *Client) findVisibleTargets(root, cache *iunknown, viewport spatial.Rect) ([]target.Target, error) {
	// VARIANT_BOOL false. On Windows x64 a 16-byte VARIANT value parameter is
	// passed indirectly, so the COM ABI receives this pointer.
	conditionValue := variant{VT: vtBool}
	var condition *iunknown
	hr := call(c.automation, 23, uintptr(propertyIsOffscreen), uintptr(unsafe.Pointer(&conditionValue)), uintptr(unsafe.Pointer(&condition))) // CreatePropertyCondition
	if failed(hr) || condition == nil {
		return nil, hresult("IUIAutomation.CreatePropertyCondition(IsOffscreen)", hr)
	}
	defer release(condition)
	var contentCondition *iunknown
	hr = call(c.automation, 19, uintptr(unsafe.Pointer(&contentCondition))) // get_ContentViewCondition
	if failed(hr) || contentCondition == nil {
		return nil, hresult("IUIAutomation.get_ContentViewCondition", hr)
	}
	defer release(contentCondition)
	hyperlinkValue := variant{VT: vtI4, Val: 50005}
	var hyperlinkCondition *iunknown
	hr = call(c.automation, 23, uintptr(propertyControlType), uintptr(unsafe.Pointer(&hyperlinkValue)), uintptr(unsafe.Pointer(&hyperlinkCondition)))
	if failed(hr) || hyperlinkCondition == nil {
		return nil, hresult("IUIAutomation.CreatePropertyCondition(Hyperlink)", hr)
	}
	defer release(hyperlinkCondition)

	var contentOrHyperlink *iunknown
	hr = call(c.automation, 27,
		uintptr(unsafe.Pointer(contentCondition)),
		uintptr(unsafe.Pointer(hyperlinkCondition)),
		uintptr(unsafe.Pointer(&contentOrHyperlink)),
	) // CreateOrCondition
	if failed(hr) || contentOrHyperlink == nil {
		return nil, hresult("IUIAutomation.CreateOrCondition", hr)
	}
	defer release(contentOrHyperlink)
	focusableValue := variant{VT: vtBool, Val: 0xFFFF}
	var focusableCondition *iunknown
	hr = call(c.automation, 23, uintptr(propertyIsKeyboardFocusable), uintptr(unsafe.Pointer(&focusableValue)), uintptr(unsafe.Pointer(&focusableCondition)))
	if failed(hr) || focusableCondition == nil {
		return nil, hresult("IUIAutomation.CreatePropertyCondition(KeyboardFocusable)", hr)
	}
	defer release(focusableCondition)

	var semanticCondition *iunknown
	hr = call(c.automation, 27,
		uintptr(unsafe.Pointer(contentOrHyperlink)),
		uintptr(unsafe.Pointer(focusableCondition)),
		uintptr(unsafe.Pointer(&semanticCondition)),
	) // CreateOrCondition
	if failed(hr) || semanticCondition == nil {
		return nil, hresult("IUIAutomation.CreateOrCondition(Focusable)", hr)
	}
	defer release(semanticCondition)

	var combinedCondition *iunknown
	hr = call(c.automation, 25,
		uintptr(unsafe.Pointer(condition)),
		uintptr(unsafe.Pointer(semanticCondition)),
		uintptr(unsafe.Pointer(&combinedCondition)),
	) // CreateAndCondition
	if failed(hr) || combinedCondition == nil {
		return nil, hresult("IUIAutomation.CreateAndCondition", hr)
	}
	defer release(combinedCondition)

	var elements *iunknown
	hr = call(root, 8, treeScopeDescendants, uintptr(unsafe.Pointer(combinedCondition)), uintptr(unsafe.Pointer(cache)), uintptr(unsafe.Pointer(&elements))) // FindAllBuildCache
	if failed(hr) || elements == nil {
		return nil, hresult("IUIAutomationElement.FindAllBuildCache", hr)
	}
	defer release(elements)

	var length int32
	hr = call(elements, 3, uintptr(unsafe.Pointer(&length))) // get_Length
	if failed(hr) {
		return nil, hresult("IUIAutomationElementArray.get_Length", hr)
	}
	out := make([]target.Target, 0, min(int(length), interactiveTargetLimit))
	for index := int32(0); index < length && len(out) < interactiveTargetLimit; index++ {
		var el *iunknown
		if failed(call(elements, 4, uintptr(index), uintptr(unsafe.Pointer(&el)))) || el == nil { // GetElement
			continue
		}
		bounds := cachedRectValue(el, propertyBoundingRectangle)
		if bounds.W > 0 && bounds.H > 0 && rectsIntersect(bounds, viewport) {
			ct := cachedInt32Value(el, propertyControlType)
			t := readCachedInteractiveTarget(el, 0, ct, bounds, false)
			if t.Actionable() {
				out = append(out, t)
			}
		}
		release(el)
	}
	return out, nil
}

func (c *Client) interactiveCacheRequest() (*iunknown, error) {
	var cache *iunknown
	hr := call(c.automation, 20, uintptr(unsafe.Pointer(&cache))) // CreateCacheRequest
	if failed(hr) || cache == nil {
		return nil, hresult("IUIAutomation.CreateCacheRequest", hr)
	}
	properties := [...]int{
		propertyControlType, propertyBoundingRectangle, propertyIsOffscreen,
		propertyName, propertyAutomationID, propertyClassName,
		propertyIsEnabled, propertyIsKeyboardFocusable,
		propertyIsInvokeAvailable, propertyIsSelectionAvailable,
		propertyIsToggleAvailable, propertyLegacyDefaultAction,
	}
	for _, propertyID := range properties {
		if hr = call(cache, 3, uintptr(propertyID)); failed(hr) { // AddProperty
			release(cache)
			return nil, hresult("IUIAutomationCacheRequest.AddProperty", hr)
		}
	}
	if hr = call(cache, 7, treeScopeElement); failed(hr) { // put_TreeScope
		release(cache)
		return nil, hresult("IUIAutomationCacheRequest.put_TreeScope", hr)
	}
	return cache, nil
}

// Interactive hint discovery is intentionally bounded. Chromium exposes the
// complete document accessibility tree, which can contain tens of thousands
// of nodes on a long article. Walking that entire tree makes activation depend
// on page length and commonly hits the process timeout. Inspector uses the
// separate exhaustive walker below.
const (
	interactiveNodeLimit   = 2500
	interactiveTargetLimit = 400
)

type actionableWalkState struct{ visited int }

func (c *Client) walkActionable(walker, cache, parent *iunknown, depth int, viewport spatial.Rect, out *[]target.Target, state *actionableWalkState) {
	if depth > 64 || state.visited >= interactiveNodeLimit || len(*out) >= interactiveTargetLimit {
		return
	}
	var child *iunknown
	// BuildCache returns the element and all interactive properties in one
	// provider round trip. Subsequent property reads are local COM cache reads.
	if failed(call(walker, 10, uintptr(unsafe.Pointer(parent)), uintptr(unsafe.Pointer(cache)), uintptr(unsafe.Pointer(&child)))) {
		return
	}
	for child != nil {
		if state.visited >= interactiveNodeLimit || len(*out) >= interactiveTargetLimit {
			release(child)
			return
		}
		state.visited++
		bounds := cachedRectValue(child, propertyBoundingRectangle)
		offscreen := cachedBoolValue(child, propertyIsOffscreen)
		hasBounds := bounds.W > 0 && bounds.H > 0
		intersectsViewport := hasBounds && rectsIntersect(bounds, viewport)
		visibleElement := !offscreen && intersectsViewport
		if visibleElement {
			controlType := cachedInt32Value(child, propertyControlType)
			if roleFor(controlType) != target.RoleUnknown {
				t := readCachedInteractiveTarget(child, depth, controlType, bounds, offscreen)
				if t.Actionable() {
					*out = append(*out, t)
				}
			}
		}
		// UIA does not guarantee that an offscreen/empty parent has no visible
		// descendants. Chromium uses such intermediate containers around web
		// content (including search-result links). Prune only when two strong
		// signals agree: a valid parent rectangle is both offscreen and wholly
		// outside the viewport. Empty or intersecting containers are traversed.
		pruneBranch := offscreen && hasBounds && !intersectsViewport
		if !pruneBranch {
			c.walkActionable(walker, cache, child, depth+1, viewport, out, state)
		}
		var next *iunknown
		call(walker, 12, uintptr(unsafe.Pointer(child)), uintptr(unsafe.Pointer(cache)), uintptr(unsafe.Pointer(&next)))
		release(child)
		child = next
		if state.visited >= interactiveNodeLimit || len(*out) >= interactiveTargetLimit {
			if child != nil {
				release(child)
			}
			return
		}
	}
}

func readCachedInteractiveTarget(el *iunknown, depth int, ct int32, bounds spatial.Rect, offscreen bool) target.Target {
	t := target.Target{
		Name: cachedStringValue(el, propertyName), AutomationID: cachedStringValue(el, propertyAutomationID),
		ClassName: cachedStringValue(el, propertyClassName), Bounds: bounds,
		Enabled: cachedBoolValue(el, propertyIsEnabled), Focusable: cachedBoolValue(el, propertyIsKeyboardFocusable),
		Offscreen: offscreen, Source: target.SourceUIAutomation, Depth: depth,
		ControlType: controlTypeName(ct), Role: roleFor(ct),
	}
	t.ID = t.AutomationID
	if t.ID == "" {
		t.ID = "uia-" + strconv.Itoa(depth) + "-" + strconv.Itoa(int(t.Bounds.X)) + "-" + strconv.Itoa(int(t.Bounds.Y))
	}
	knownInteractiveRole := t.Role != target.RoleUnknown
	patternAvailable := cachedBoolValue(el, propertyIsInvokeAvailable) ||
		cachedBoolValue(el, propertyIsSelectionAvailable) ||
		cachedBoolValue(el, propertyIsToggleAvailable)
	legacyClickable := cachedStringValue(el, propertyLegacyDefaultAction) != ""
	// Chrome often exposes web links as Custom/Group/Text controls carrying an
	// Invoke flag or a LegacyIAccessible default action. UIA is discovery-only;
	// all selected targets are still activated by a real coordinate click.
	if t.Enabled && (knownInteractiveRole || patternAvailable || legacyClickable || t.Focusable) {
		t.Actions = []target.Action{target.ActionClick}
	}
	return t
}

func rectsIntersect(a, b spatial.Rect) bool {
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

// ExecuteTarget applies the same isolation to potentially blocking patterns.
func ExecuteTarget(want target.Target, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case executionSlot <- struct{}{}:
	case <-timer.C:
		return ErrActionBusy
	}
	ch := make(chan error, 1)
	go func() {
		defer func() { <-executionSlot }()
		client, err := New()
		if err == nil {
			err = client.ExecuteForeground(want)
			client.Close()
		}
		ch <- err
	}()
	select {
	case err := <-ch:
		return err
	case <-timer.C:
		return fmt.Errorf("%w after %s", ErrActionTimeout, timeout)
	}
}

func New() (*Client, error) {
	runtime.LockOSThread()
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if failed(hr) {
		runtime.UnlockOSThread()
		return nil, hresult("CoInitializeEx", hr)
	}
	var automation *iunknown
	hr, _, _ = procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidCUIAutomation)), 0, clsctxInprocServer, uintptr(unsafe.Pointer(&iidIUIAutomation)), uintptr(unsafe.Pointer(&automation)))
	if failed(hr) {
		procCoUninitialize.Call()
		runtime.UnlockOSThread()
		return nil, hresult("CoCreateInstance(CUIAutomation)", hr)
	}
	return &Client{automation: automation, initialized: true}, nil
}

func (c *Client) Close() {
	if c == nil || !c.initialized {
		return
	}
	release(c.automation)
	c.automation = nil
	procCoUninitialize.Call()
	runtime.UnlockOSThread()
	c.initialized = false
}

func (c *Client) ForegroundTargets() ([]target.Target, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil, fmt.Errorf("GetForegroundWindow returned 0")
	}
	return c.Targets(hwnd)
}

func (c *Client) Targets(hwnd uintptr) ([]target.Target, error) {
	var root *iunknown
	hr := call(c.automation, 6, hwnd, uintptr(unsafe.Pointer(&root))) // ElementFromHandle
	if failed(hr) || root == nil {
		return nil, hresult("IUIAutomation.ElementFromHandle", hr)
	}
	defer release(root)
	var walker *iunknown
	hr = call(c.automation, 14, uintptr(unsafe.Pointer(&walker))) // get_ControlViewWalker
	if failed(hr) || walker == nil {
		return nil, hresult("IUIAutomation.ControlViewWalker", hr)
	}
	defer release(walker)
	var out []target.Target
	if err := c.walk(walker, root, 0, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ExecuteForeground resolves the target again and uses its best supported
// control pattern. The caller can fall back to a coordinate click on error.
func (c *Client) ExecuteForeground(want target.Target) error {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return fmt.Errorf("GetForegroundWindow returned 0")
	}
	var root *iunknown
	hr := call(c.automation, 6, hwnd, uintptr(unsafe.Pointer(&root)))
	if failed(hr) || root == nil {
		return hresult("ElementFromHandle", hr)
	}
	defer release(root)
	var walker *iunknown
	hr = call(c.automation, 14, uintptr(unsafe.Pointer(&walker)))
	if failed(hr) || walker == nil {
		return hresult("ControlViewWalker", hr)
	}
	defer release(walker)
	if executeIfMatch(root, want) {
		return nil
	}
	if c.findAndExecute(walker, root, want, 0) {
		return nil
	}
	return fmt.Errorf("UIA target %q is no longer available", want.ID)
}

func (c *Client) findAndExecute(walker, parent *iunknown, want target.Target, depth int) bool {
	if depth > 64 {
		return false
	}
	var child *iunknown
	if failed(call(walker, 4, uintptr(unsafe.Pointer(parent)), uintptr(unsafe.Pointer(&child)))) {
		return false
	}
	for child != nil {
		if executeIfMatch(child, want) {
			release(child)
			return true
		}
		if c.findAndExecute(walker, child, want, depth+1) {
			release(child)
			return true
		}
		var next *iunknown
		call(walker, 6, uintptr(unsafe.Pointer(child)), uintptr(unsafe.Pointer(&next)))
		release(child)
		child = next
	}
	return false
}

func executeIfMatch(el *iunknown, want target.Target) bool {
	got := readTarget(el, want.Depth)
	if want.AutomationID != "" {
		if got.AutomationID != want.AutomationID || got.Name != want.Name || got.ControlType != want.ControlType || got.ClassName != want.ClassName || got.Bounds != want.Bounds {
			return false
		}
	} else if got.Name != want.Name || got.ControlType != want.ControlType || got.Bounds != want.Bounds {
		return false
	}
	for _, action := range want.Actions {
		switch action {
		case target.ActionInvoke:
			if invokePattern(el, patternInvoke) {
				return true
			}
		case target.ActionToggle:
			if invokePattern(el, patternToggle) {
				return true
			}
		case target.ActionSelect:
			if invokePattern(el, patternSelectionItem) {
				return true
			}
		case target.ActionFocus:
			return !failed(call(el, 3))
		}
	}
	return false
}

func invokePattern(el *iunknown, patternID int) bool {
	var pattern *iunknown
	hr := call(el, 16, uintptr(patternID), uintptr(unsafe.Pointer(&pattern)))
	if failed(hr) || pattern == nil {
		return false
	}
	defer release(pattern)
	return !failed(call(pattern, 3))
}

func (c *Client) walk(walker, parent *iunknown, depth int, out *[]target.Target) error {
	if depth > 64 || len(*out) >= 10000 {
		return nil
	}
	var child *iunknown
	hr := call(walker, 4, uintptr(unsafe.Pointer(parent)), uintptr(unsafe.Pointer(&child)))
	if failed(hr) {
		return nil
	}
	for child != nil {
		t := readTarget(child, depth)
		if t.Bounds.W > 0 && t.Bounds.H > 0 {
			*out = append(*out, t)
		}
		_ = c.walk(walker, child, depth+1, out)
		var next *iunknown
		hr = call(walker, 6, uintptr(unsafe.Pointer(child)), uintptr(unsafe.Pointer(&next)))
		release(child)
		child = next
		if failed(hr) {
			break
		}
	}
	return nil
}

func readTarget(el *iunknown, depth int) target.Target {
	ct := int32Value(el, propertyControlType)
	return readTargetKnown(el, depth, ct, rectValue(el), boolValue(el, propertyIsOffscreen))
}

// readTargetKnown reuses the viewport-filter properties already fetched by
// the interactive walker, avoiding three extra cross-process calls per node.
func readTargetKnown(el *iunknown, depth int, ct int32, bounds spatial.Rect, offscreen bool) target.Target {
	t := target.Target{
		Name: stringValue(el, propertyName), AutomationID: stringValue(el, propertyAutomationID),
		ClassName: stringValue(el, propertyClassName), Bounds: bounds,
		Enabled: boolValue(el, propertyIsEnabled), Focusable: boolValue(el, propertyIsKeyboardFocusable),
		Offscreen: offscreen, Source: target.SourceUIAutomation, Depth: depth,
		ControlType: controlTypeName(ct), Role: roleFor(ct),
	}
	t.ID = t.AutomationID
	if t.ID == "" {
		t.ID = "uia-" + strconv.Itoa(depth) + "-" + strconv.Itoa(int(t.Bounds.X)) + "-" + strconv.Itoa(int(t.Bounds.Y))
	}
	// Pattern lookups are cross-process COM calls. Query only patterns that are
	// meaningful for this control type instead of making three calls for every
	// text, pane and decorative node in the tree.
	switch t.Role {
	case target.RoleButton, target.RoleHyperlink, target.RoleMenuItem:
		if supportsPattern(el, patternInvoke) {
			t.Actions = append(t.Actions, target.ActionInvoke)
		}
	case target.RoleCheckBox:
		if supportsPattern(el, patternToggle) {
			t.Actions = append(t.Actions, target.ActionToggle)
		}
	case target.RoleListItem, target.RoleRadio, target.RoleTabItem, target.RoleTreeItem:
		if supportsPattern(el, patternSelectionItem) {
			t.Actions = append(t.Actions, target.ActionSelect)
		}
	case target.RoleComboBox:
		if supportsPattern(el, patternInvoke) {
			t.Actions = append(t.Actions, target.ActionInvoke)
		}
	}
	if t.Focusable {
		t.Actions = append(t.Actions, target.ActionFocus)
	}
	if len(t.Actions) == 0 && t.Enabled {
		t.Actions = append(t.Actions, target.ActionClick)
	}
	return t
}

func property(el *iunknown, id int) variant {
	var v variant
	call(el, 10, uintptr(id), uintptr(unsafe.Pointer(&v)))
	return v
}

func cachedProperty(el *iunknown, id int) variant {
	var v variant
	call(el, 12, uintptr(id), uintptr(unsafe.Pointer(&v))) // GetCachedPropertyValue
	return v
}
func clear(v *variant) { procVariantClear.Call(uintptr(unsafe.Pointer(v))) }

func cachedStringValue(el *iunknown, id int) string {
	v := cachedProperty(el, id)
	defer clear(&v)
	if v.VT != vtBSTR || v.Val == 0 {
		return ""
	}
	bstr := *(**uint16)(unsafe.Pointer(&v.Val))
	return windows.UTF16PtrToString(bstr)
}

func cachedInt32Value(el *iunknown, id int) int32 {
	v := cachedProperty(el, id)
	defer clear(&v)
	if v.VT != vtI4 {
		return 0
	}
	return int32(v.Val)
}

func cachedBoolValue(el *iunknown, id int) bool {
	v := cachedProperty(el, id)
	defer clear(&v)
	return v.VT == vtBool && int16(v.Val) != 0
}

func cachedRectValue(el *iunknown, id int) spatial.Rect {
	v := cachedProperty(el, id)
	defer clear(&v)
	return rectFromVariant(v)
}
func stringValue(el *iunknown, id int) string {
	v := property(el, id)
	defer clear(&v)
	if v.VT != vtBSTR || v.Val == 0 {
		return ""
	}
	bstr := *(**uint16)(unsafe.Pointer(&v.Val))
	return windows.UTF16PtrToString(bstr)
}
func int32Value(el *iunknown, id int) int32 {
	v := property(el, id)
	defer clear(&v)
	if v.VT != vtI4 {
		return 0
	}
	return int32(v.Val)
}
func boolValue(el *iunknown, id int) bool {
	v := property(el, id)
	defer clear(&v)
	return v.VT == vtBool && int16(v.Val) != 0
}
func rectValue(el *iunknown) spatial.Rect {
	v := property(el, propertyBoundingRectangle)
	defer clear(&v)
	return rectFromVariant(v)
}

func rectFromVariant(v variant) spatial.Rect {
	if v.VT != vtArray|vtR8 || v.Val == 0 {
		return spatial.Rect{}
	}
	sa := uintptr(v.Val)
	var lo, hi int32
	var data unsafe.Pointer
	if r, _, _ := procSafeArrayGetLBound.Call(sa, 1, uintptr(unsafe.Pointer(&lo))); failed(r) {
		return spatial.Rect{}
	}
	if r, _, _ := procSafeArrayGetUBound.Call(sa, 1, uintptr(unsafe.Pointer(&hi))); failed(r) || hi-lo+1 < 4 {
		return spatial.Rect{}
	}
	if r, _, _ := procSafeArrayAccessData.Call(sa, uintptr(unsafe.Pointer(&data))); failed(r) {
		return spatial.Rect{}
	}
	defer procSafeArrayUnaccessData.Call(sa)
	values := unsafe.Slice((*float64)(data), hi-lo+1)
	return spatial.Rect{X: values[0], Y: values[1], W: values[2], H: values[3]}
}
func supportsPattern(el *iunknown, pattern int) bool {
	var p *iunknown
	hr := call(el, 16, uintptr(pattern), uintptr(unsafe.Pointer(&p)))
	if p != nil {
		release(p)
	}
	return !failed(hr) && p != nil
}

func roleFor(id int32) target.Role {
	switch id {
	case 50000, 50031:
		return target.RoleButton
	case 50001:
		return target.RoleCalendar
	case 50002:
		return target.RoleCheckBox
	case 50003:
		return target.RoleComboBox
	case 50004:
		return target.RoleEdit
	case 50005:
		return target.RoleHyperlink
	case 50007:
		return target.RoleListItem
	case 50011:
		return target.RoleMenuItem
	case 50013:
		return target.RoleRadio
	case 50019:
		return target.RoleTabItem
	case 50024:
		return target.RoleTreeItem
	}
	return target.RoleUnknown
}
func controlTypeName(id int32) string {
	names := map[int32]string{50000: "Button", 50001: "Calendar", 50002: "CheckBox", 50003: "ComboBox", 50004: "Edit", 50005: "Hyperlink", 50006: "Image", 50007: "ListItem", 50008: "List", 50009: "Menu", 50010: "MenuBar", 50011: "MenuItem", 50012: "ProgressBar", 50013: "RadioButton", 50014: "ScrollBar", 50015: "Slider", 50016: "Spinner", 50017: "StatusBar", 50018: "Tab", 50019: "TabItem", 50020: "Text", 50021: "ToolBar", 50022: "ToolTip", 50023: "Tree", 50024: "TreeItem", 50025: "Custom", 50026: "Group", 50027: "Thumb", 50028: "DataGrid", 50029: "DataItem", 50030: "Document", 50031: "SplitButton", 50032: "Window", 50033: "Pane", 50034: "Header", 50035: "HeaderItem", 50036: "Table", 50037: "TitleBar", 50038: "Separator"}
	if n := names[id]; n != "" {
		return n
	}
	return "Unknown"
}

func call(obj *iunknown, index uintptr, args ...uintptr) uintptr {
	if obj == nil || obj.vtbl == nil {
		return ^uintptr(0)
	}
	fn := *(*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(obj.vtbl)) + index*unsafe.Sizeof(uintptr(0))))
	argv := append([]uintptr{uintptr(unsafe.Pointer(obj))}, args...)
	r, _, _ := syscall.SyscallN(fn, argv...)
	return r
}
func release(obj *iunknown) {
	if obj != nil {
		call(obj, 2)
	}
}
func failed(hr uintptr) bool { return int32(hr) < 0 }
func hresult(op string, hr uintptr) error {
	return fmt.Errorf("%s failed: HRESULT 0x%08X", op, uint32(hr))
}
