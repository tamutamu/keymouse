//go:build windows

// keymouse は、キーボードだけで画面上の任意の位置をクリックするための
// Windows 常駐アプリケーションのエントリポイントである。
package main

import (
	"log"
	"os"
	"runtime"

	"github.com/tamutamu/keymouse/internal/app"
	"github.com/tamutamu/keymouse/internal/settings"
)

func main() {
	// Win32 のウィンドウ・ホットキー・メッセージループは、それらを生成した
	// 同一の OS スレッド上で実行しなければならない。Go ランタイムは goroutine を
	// 任意のタイミングで別スレッドへ移し得るため、メインスレッドを固定する。
	// これを怠ると RegisterHotKey が "belongs to other thread" で失敗したり、
	// メッセージループがウィンドウメッセージを受け取れなくなる。
	runtime.LockOSThread()

	// ログはファイル名・行番号付きで標準エラー出力へ。
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stderr)

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
