//go:build windows

// Package tray はタスクトレイ常駐アイコンと、その右クリックメニュー
// (設定表示・終了)を提供する。Win32 呼び出しはすべて win32 パッケージ経由で行う。
package tray

import (
	"golang.org/x/sys/windows"

	"github.com/tamutamu/keymouse/internal/win32"
)

const (
	// wmTrayMsg はトレイアイコンのマウス操作を受け取るための独自コールバックメッセージ
	// (WM_USER + 1)。ウィンドウプロシージャはこの値を HandleMessage に委譲する。
	wmTrayMsg = 0x0400 + 1

	idTrayIcon = 1

	// 右クリックメニューのコマンドID
	idMenuSettings = 100
	idMenuExit     = 101
)

// Tray はタスクトレイアイコンを管理する。
type Tray struct {
	hwnd windows.HWND
	nid  *win32.NOTIFYICONDATA

	// OnSettings は「設定」選択時、OnExit は「終了」選択時に呼ばれる。
	OnSettings func()
	OnExit     func()
}

// New は指定した HWND に紐づく Tray を生成する。
// その HWND のウィンドウプロシージャは wmTrayMsg を HandleMessage へ委譲すること。
func New(hwnd uintptr) *Tray {
	return &Tray{hwnd: windows.HWND(hwnd)}
}

// Add は通知領域にトレイアイコンを追加する。
func (t *Tray) Add() error {
	nid, err := win32.AddTrayIcon(t.hwnd, idTrayIcon, wmTrayMsg, "KeyMouse")
	if err != nil {
		return err
	}
	t.nid = nid
	return nil
}

// Remove はトレイアイコンを削除する。
func (t *Tray) Remove() {
	win32.RemoveTrayIcon(t.nid)
}

// HandleMessage は wmTrayMsg を処理する。ウィンドウプロシージャから呼び出すこと。
// 自分宛のメッセージでなければ (false, 0) を返す。
func (t *Tray) HandleMessage(hwnd uintptr, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
	if msg != wmTrayMsg {
		return false, 0
	}
	switch lParam & 0xFFFF {
	case win32.WM_RBUTTONUP:
		t.showContextMenu(hwnd)
	case win32.WM_LBUTTONDBLCLK:
		if t.OnSettings != nil {
			t.OnSettings()
		}
	}
	return true, 0
}

// showContextMenu は右クリック時のコンテキストメニュー(設定/終了)を表示し、
// 選択結果に応じてコールバックを呼ぶ。
func (t *Tray) showContextMenu(hwnd uintptr) {
	menu := win32.NewPopupMenu()
	if menu == nil {
		return
	}
	defer menu.Destroy()

	menu.AppendStringItem(idMenuSettings, "Settings...")
	menu.AppendStringItem(idMenuExit, "Exit")

	switch menu.TrackAtCursor(hwnd) {
	case idMenuSettings:
		if t.OnSettings != nil {
			t.OnSettings()
		}
	case idMenuExit:
		if t.OnExit != nil {
			t.OnExit()
		}
	}
}
