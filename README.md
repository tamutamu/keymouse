# keymouse

keymouse is a Windows tray app for clicking any point on the screen using only
the keyboard. It opens a lightweight overlay on the monitor under the cursor,
narrows the target area through labeled grid selections, and sends a left,
right, or double click at the selected point.

## Features

- Keyboard-only pointer targeting for Windows
- Left-click targeting with a quick double-tap of `Shift`
- Global hotkeys for right click (`Alt+R`) and double click (`Alt+D`)
- Hierarchical `5 x 5` label grid using single-letter labels (`A` through `Y`)
- `Shift` + label to click immediately at the current grid level
- `Backspace` to go back one level, `Esc` to cancel
- `Alt+H/J/K/L` to nudge the current grid left/down/up/right
- Per-monitor operation based on the monitor currently under the cursor
- Tray icon with Settings and Exit actions
- Optional Windows startup registration
- Local-only behavior; no telemetry or network communication

## Requirements

- Windows 11
- Windows 10 may work, but Windows 11 is the primary target
- Go 1.22 or later to build from source

## Build

From PowerShell:

```powershell
.\build.ps1
```

Or with `go build` directly:

```powershell
go build -ldflags "-H windowsgui" -o keymouse.exe ./cmd/app
```

The `windowsgui` subsystem prevents a console window from appearing when the
tray app starts.

## Usage

1. Start `keymouse.exe`.
2. Use the tray icon to open Settings or Exit.
3. Double-tap `Shift` to start a left-click selection.
4. Press a label key (`A` through `Y`) to zoom into that area.
5. Press `Shift` + label to click immediately, or continue selecting labels
   until the final level clicks automatically.
6. Use `Alt+R` for a right-click selection, and `Alt+D` for a double-click
   selection.

During selection:

- `Backspace`: return to the previous level
- `Esc`: cancel
- `Alt+H`: move the current grid left
- `Alt+J`: move the current grid down
- `Alt+K`: move the current grid up
- `Alt+L`: move the current grid right

## Configuration

Settings are stored at:

```text
%APPDATA%\keymouse\config.json
```

The Settings window currently exposes Windows startup registration. Other
behavior, such as hotkeys, grid size, depth, and label size, can be adjusted by
editing `config.json`.

Default values include:

- Grid: `5 x 5`
- Maximum depth: `3`
- Right click hotkey: `Alt+R`
- Double click hotkey: `Alt+D`
- Startup registration: disabled

Hotkey and grid changes should be made while the app is not running, or applied
after restarting the app.

## Development

Run tests:

```powershell
go test ./...
```

Build:

```powershell
.\build.ps1
```

The main entry point is `cmd/app`, with implementation packages under
`internal/`.

## License

MIT

## 日本語

keymouse は、キーボードだけで Windows 画面上の任意の位置を指定してクリックする
タスクトレイ常駐アプリです。カーソルがあるモニター上に軽量なオーバーレイを表示し、
ラベル付きグリッドを段階的に絞り込んで、左クリック、右クリック、ダブルクリックを
送信します。

### 特徴

- Windows 向けのキーボード操作によるポインター指定
- `Shift` 2連打で左クリック選択を開始
- 右クリックは `Alt+R`、ダブルクリックは `Alt+D`
- `A` から `Y` までの 1 文字ラベルを使う階層型 `5 x 5` グリッド
- `Shift` + ラベルで現在の階層から即クリック
- `Backspace` で 1 段戻る、`Esc` でキャンセル
- `Alt+H/J/K/L` で現在のグリッドを左/下/上/右へ微調整
- カーソル位置のモニターを対象に動作
- タスクトレイから Settings と Exit を実行
- Windows 起動時の自動起動に対応
- ローカル完結。テレメトリやネットワーク送信はありません

### 要件

- Windows 11
- Windows 10 でも動作する可能性がありますが、主な対象は Windows 11 です
- ソースからビルドする場合は Go 1.22 以上

### ビルド

PowerShell から実行します。

```powershell
.\build.ps1
```

または、直接 `go build` を実行します。

```powershell
go build -ldflags "-H windowsgui" -o keymouse.exe ./cmd/app
```

`windowsgui` サブシステムでビルドすることで、起動時にコンソールウィンドウが
表示されません。

### 使い方

1. `keymouse.exe` を起動します。
2. タスクトレイアイコンから Settings または Exit を実行できます。
3. `Shift` を 2 連打すると左クリック選択が始まります。
4. ラベルキー（`A` から `Y`）を押して対象領域を絞り込みます。
5. `Shift` + ラベルで即クリックするか、最終階層まで選択すると自動クリックします。
6. 右クリック選択は `Alt+R`、ダブルクリック選択は `Alt+D` です。

選択中の操作:

- `Backspace`: 1 段戻る
- `Esc`: キャンセル
- `Alt+H`: 現在のグリッドを左へ移動
- `Alt+J`: 現在のグリッドを下へ移動
- `Alt+K`: 現在のグリッドを上へ移動
- `Alt+L`: 現在のグリッドを右へ移動

### 設定

設定ファイルは次の場所に保存されます。

```text
%APPDATA%\keymouse\config.json
```

Settings ウィンドウでは、現在 Windows 起動時の自動起動を設定できます。ホットキー、
グリッドサイズ、階層数、ラベルサイズなどは `config.json` を編集して変更できます。

主な初期値:

- グリッド: `5 x 5`
- 最大階層数: `3`
- 右クリックホットキー: `Alt+R`
- ダブルクリックホットキー: `Alt+D`
- 自動起動: 無効

ホットキーやグリッド設定を変更した場合は、アプリの再起動後に反映されます。

### 開発

テスト:

```powershell
go test ./...
```

ビルド:

```powershell
.\build.ps1
```

エントリポイントは `cmd/app`、実装パッケージは `internal/` 以下にあります。
