# keymouse

キーボードだけで Windows 画面上の任意の位置を高速・予測可能に指定してクリックする OSS ツール。

## 特徴

- **グローバルホットキー**で起動（`Alt+Space`=左クリック、`Alt+R`=右クリック、`Alt+D`=ダブルクリック）
- 画面を階層的に分割し **1文字ラベル** だけを表示（グリッド線なし）
- 通常入力で絞り込み・拡大、**Shift付き**で即クリック
- `Backspace` で一段戻る、`Esc` でキャンセル
- Per-Monitor DPI Aware（Full HD / WQHD / 4K 対応）
- タスクトレイ常駐、Windows 自動起動対応
- ローカル完結（ネットワーク送信・テレメトリなし）

## 要件

- Windows 11（Windows 10 でも動作する可能性あり）
- Go 1.22 以上（ビルドのみ）

## ビルド

```bash
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o keymouse.exe ./cmd/app
```

## 使い方

1. `keymouse.exe` を起動するとタスクトレイに常駐
2. `Alt+Space` を押してオーバーレイを起動
3. ラベルキーを入力して絞り込み → Shift+ラベルでクリック
4. `Backspace` で一段戻る、`Esc` でキャンセル

## ライセンス

MIT
