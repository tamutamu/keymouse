//go:build windows

// keymouse は、キーボードだけで画面上の任意の位置をクリックするための
// Windows 常駐アプリケーションのエントリポイントである。
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/tamutamu/keymouse/internal/app"
	"github.com/tamutamu/keymouse/internal/inspector"
	"github.com/tamutamu/keymouse/internal/settings"
	"github.com/tamutamu/keymouse/internal/uia"
)

func main() {
	// Win32 のウィンドウ・ホットキー・メッセージループは、それらを生成した
	// 同一の OS スレッド上で実行しなければならない。Go ランタイムは goroutine を
	// 任意のタイミングで別スレッドへ移し得るため、メインスレッドを固定する。
	// これを怠ると RegisterHotKey が "belongs to other thread" で失敗したり、
	// メッセージループがウィンドウメッセージを受け取れなくなる。
	runtime.LockOSThread()

	if len(os.Args) >= 2 && os.Args[1] == uia.WorkerScanArgument {
		var hwnd uintptr
		if len(os.Args) >= 3 {
			value, err := strconv.ParseUint(os.Args[2], 0, 64)
			if err != nil {
				fmt.Fprintln(os.Stderr, "invalid hwnd:", err)
				os.Exit(1)
			}
			hwnd = uintptr(value)
		}
		if err := uia.RunScanWorker(os.Stdout, hwnd); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == uia.WorkerExecuteArgument {
		if err := uia.RunExecuteWorker(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Inspector is intentionally part of the same binary so it exercises the
	// exact UIA engine used by Element Hint mode.
	if len(os.Args) >= 2 && os.Args[1] == "inspect" {
		var err error
		if len(os.Args) >= 4 && os.Args[2] == "--hwnd" {
			value, parseErr := strconv.ParseUint(os.Args[3], 0, 64)
			if parseErr != nil {
				fmt.Fprintln(os.Stderr, "invalid hwnd:", parseErr)
				os.Exit(1)
			}
			err = inspector.RunWindow(os.Stdout, true, uintptr(value))
		} else {
			err = inspector.Run(os.Stdout, true)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "inspect:", err)
			os.Exit(1)
		}
		return
	}

	// GUI アプリでも候補検出の失敗を調べられるよう、実行ファイルの隣に
	// 診断ログを残す。起動に失敗しても従来通り標準エラーへフォールバックする。
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if exe, err := os.Executable(); err == nil {
		if f, err := os.OpenFile(filepath.Join(filepath.Dir(exe), "keymouse.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			defer f.Close()
			log.SetOutput(f)
		} else {
			log.SetOutput(os.Stderr)
		}
	} else {
		log.SetOutput(os.Stderr)
	}

	// 設定を読み込む(存在しなければ既定値、破損していれば退避して既定値)。
	cfg, err := settings.Load()
	if err != nil {
		log.Printf("settings load error (using defaults): %v", err)
	}

	// アプリケーションを生成して実行する(メッセージループで終了までブロックする)。
	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("app init: %v", err)
	}

	if err := a.Run(); err != nil {
		log.Fatalf("app run: %v", err)
	}
}
