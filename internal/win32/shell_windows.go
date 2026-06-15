//go:build windows

package win32

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 本ファイルはタスクトレイ(Shell_NotifyIcon)、ポップアップメニュー、
// メッセージボックスといった User32/Shell32 機能の型付きラッパーを提供する。
// これにより tray / tutorial / settings パッケージが DLL を直接ロードせずに済む。

const (
	// NOTIFYICONDATA の dwMessage 用
	NIM_ADD    = 0
	NIM_MODIFY = 1
	NIM_DELETE = 2

	// NOTIFYICONDATA の uFlags 用
	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	// TrackPopupMenu のフラグ
	TPM_RIGHTALIGN  = 0x0008
	TPM_BOTTOMALIGN = 0x0020
	TPM_RETURNCMD   = 0x0100

	// AppendMenu のフラグ
	MF_STRING  = 0x0000
	MF_ENABLED = 0x0000
	MF_GRAYED  = 0x0001

	// 既定のアプリケーションアイコン ID
	IDI_APPLICATION = 32512

	// MessageBox のフラグ
	MB_OK              = 0x00000000
	MB_ICONERROR       = 0x00000010
	MB_ICONINFORMATION = 0x00000040

	// トレイアイコンのマウス通知 (lParam に格納される)
	WM_RBUTTONUP     = 0x0205
	WM_LBUTTONDBLCLK = 0x0203
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
	procCreatePopupMenu = user32.NewProc("CreatePopupMenu")
	procAppendMenu      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu  = user32.NewProc("TrackPopupMenu")
	procDestroyMenu     = user32.NewProc("DestroyMenu")
	procLoadIcon        = user32.NewProc("LoadIconW")
	procMessageBox      = user32.NewProc("MessageBoxW")
)

// NOTIFYICONDATA は Shell_NotifyIcon に渡す構造体である。
type NOTIFYICONDATA struct {
	CbSize           uint32
	Hwnd             windows.HWND
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            windows.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeoutVersion  uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     windows.Handle
}

// LoadAppIcon は既定のアプリケーションアイコンハンドルを返す。
func LoadAppIcon() windows.Handle {
	hIcon, _, _ := procLoadIcon.Call(0, uintptr(IDI_APPLICATION))
	return windows.Handle(hIcon)
}

// AddTrayIcon は通知領域(タスクトレイ)にアイコンを追加する。
// hwnd には通知メッセージを受け取るウィンドウ、callbackMsg にはそのメッセージ値を渡す。
// 戻り値の NOTIFYICONDATA は RemoveTrayIcon で削除する際に再利用する。
func AddTrayIcon(hwnd windows.HWND, uid, callbackMsg uint32, tip string) (*NOTIFYICONDATA, error) {
	nid := &NOTIFYICONDATA{
		CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATA{})),
		Hwnd:             hwnd,
		UID:              uid,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		UCallbackMessage: callbackMsg,
		HIcon:            LoadAppIcon(),
	}
	tip16, _ := windows.UTF16FromString(tip)
	copy(nid.SzTip[:], tip16)

	r, _, err := procShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(nid)))
	if r == 0 {
		return nil, fmt.Errorf("Shell_NotifyIcon NIM_ADD: %w", err)
	}
	return nid, nil
}

// RemoveTrayIcon は通知領域からアイコンを削除する。
func RemoveTrayIcon(nid *NOTIFYICONDATA) {
	if nid == nil {
		return
	}
	procShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(nid)))
}

// PopupMenu はポップアップメニューのハンドルをラップし、項目追加・表示を補助する。
type PopupMenu struct {
	handle uintptr
}

// NewPopupMenu は空のポップアップメニューを生成する。生成失敗時は nil を返す。
func NewPopupMenu() *PopupMenu {
	h, _, _ := procCreatePopupMenu.Call()
	if h == 0 {
		return nil
	}
	return &PopupMenu{handle: h}
}

// AppendStringItem は文字列メニュー項目を追加する。id は選択時に返るコマンドID。
func (m *PopupMenu) AppendStringItem(id uintptr, text string) {
	t, _ := windows.UTF16PtrFromString(text)
	procAppendMenu.Call(m.handle, MF_STRING|MF_ENABLED, id, uintptr(unsafe.Pointer(t)))
}

// TrackAtCursor はカーソル位置にメニューを表示し、選択されたコマンドIDを返す
// (TPM_RETURNCMD 指定)。何も選択されなければ 0 を返す。
// hwnd は所有ウィンドウで、メニュー表示前に最前面化される。
func (m *PopupMenu) TrackAtCursor(hwnd uintptr) uintptr {
	x, y, _ := GetCursorPos()
	procSetForegroundWindow.Call(hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(
		m.handle,
		TPM_RIGHTALIGN|TPM_BOTTOMALIGN|TPM_RETURNCMD,
		uintptr(x), uintptr(y),
		0, hwnd, 0,
	)
	return cmd
}

// Destroy はメニューを破棄する。
func (m *PopupMenu) Destroy() {
	procDestroyMenu.Call(m.handle)
}

// MessageBox は単純なメッセージボックスを表示する。
// parent は親ウィンドウ(0 でデスクトップ)、flags は MB_* の組み合わせ。
func MessageBox(parent uintptr, text, title string, flags uint32) {
	t, _ := windows.UTF16PtrFromString(text)
	ti, _ := windows.UTF16PtrFromString(title)
	procMessageBox.Call(parent,
		uintptr(unsafe.Pointer(t)),
		uintptr(unsafe.Pointer(ti)),
		uintptr(flags))
}
