# keymouse

README: [English](README.md) | 日本語

KeyMouseは、キーボードからWindows UIの操作対象を発見し、実クリックする軽量トレイ
アプリです。Windows UI AutomationによるElement Hintと、既存の固定座標Gridを
組み合わせています。UI Automationは要素検出だけに使用し、選択後は常に物理的な
マウスクリックを実行します。

画面キャプチャ、OpenCV、テレメトリ、ネットワーク通信は使用しません。

## デモ

[![KeyMouseのElement Modeとキーボードクリックのデモ](assets/demo-preview.gif)](assets/demo.mp4)

プレビューをクリックすると、53秒のMP4デモを再生できます。

## 操作モード

`Shift+Space`で継続Element Modeを開始します。アクティブウィンドウを走査し、処理中は
`loading...`を表示して、画面内の操作可能要素へ小文字の短いラベルを付けます。候補が
少ない場合は一文字を使い、必要な場合だけ二文字、三文字へ増やします。

UIAからGridへの自動フォールバックは行いません。ラベル表示中に`Shift+G`を押すと
Grid Modeへ切り替わり、もう一度押すとUIAを再走査してElement Modeへ戻ります。
Grid Modeはカーソルのあるモニターを固定`40 × 25`に分割し、
`a s d f g t r e w q`による三文字ラベルを表示します。

## キー操作

- 小文字のラベルキー: 候補を絞り込み、確定した対象をクリック
- `Backspace`: 入力済みラベルを一文字戻す
- `H/J/K/L`: ラベルを左／下／上／右へ4px移動
- `Shift+H/J/K/L`: ラベルを16px移動
- `Shift+R`: 現在のモードを再走査してラベルを更新
- `Space`長押し: Overlayを一時的に隠す（Peek）
- `Shift+G`: Element ModeとGrid Modeを切り替え
- `Shift+Space`または`Esc`: 継続選択を終了

クリック前にOverlayを隠し、マウスポインターを対象座標へ移動して物理クリックします。
継続モードでは、その後に画面内の対象を更新して次の操作を待ちます。

## Inspector

```powershell
keymouse.exe inspect
```

フォアグラウンドウィンドウのUI AutomationツリーをJSONで出力します。Name、Control
Type、Automation ID、Class、矩形、状態、対応Action、Depthを確認でき、Provider差異や
Hint欠落の診断に使用できます。

## ビルド

```powershell
.\build.ps1
go test ./...
```

## 要件

- Windows 10 または11
- ソースビルドにはGo 1.22以上

## License

MIT
