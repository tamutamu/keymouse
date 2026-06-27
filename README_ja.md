# keymouse

README: [English](README.md) | 日本語

Windows で、マウスに手を伸ばさずにクリックする。

![keymouse demo](assets/keymouse-demo.gif)

keymouse は、キーボード中心で操作したい人向けの小さな Windows タスクトレイ常駐
アプリです。`Shift` を 2 連打し、グリッドのラベルを選ぶだけで、画面上の任意の
位置に左クリック、右クリック、ダブルクリックを送信できます。

テレメトリなし。ネットワーク通信なし。アカウント不要。ローカルで完結する
keyboard-first なマウス補助ツールです。

## Why?

Vimium、ターミナル、各種キーボードショートカットを使っていても、デスクトップ
アプリではどうしてもマウスに手を伸ばす場面があります。

keymouse は Windows にシンプルな再帰グリッドクリックを追加します。ホットキーを
押し、対象領域を絞り込み、クリックするだけです。

## インストール

[Releases](https://github.com/tamutamu/keymouse/releases) から最新の
`keymouse-vX.Y.Z-windows-amd64.zip` をダウンロードし、展開して `keymouse.exe` を
起動します。

起動後、`Shift` を 2 連打すると選択を開始できます。

まだ Release がない場合は、ソースからビルドしてください。

```powershell
.\build.ps1
```

## 特徴

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

## 立ち位置

keymouse は意図的に小さく作っています。

| Tool | Focus |
| --- | --- |
| keymouse | グリッドクリックに特化した軽量 Windows トレイアプリ |
| mousemaster | 高機能な mouseless 操作とキーマッピング |
| keynavish | Windows 向け keynav 互換のマウス操作 |
| Windows Mouse Keys | Windows 標準のアクセシビリティ向けカーソル移動 |

## 要件

- Windows 11
- Windows 10 でも動作する可能性がありますが、主な対象は Windows 11 です
- ソースからビルドする場合は Go 1.22 以上

## ソースからビルド

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

## 使い方

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

## 設定

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

## 開発

テスト:

```powershell
go test ./...
```

ビルド:

```powershell
.\build.ps1
```

リリース用 zip とチェックサムを作成:

```powershell
.\package-release.ps1 -Version v0.1.0
```

エントリポイントは `cmd/app`、実装パッケージは `internal/` 以下にあります。

## ライセンス

MIT
