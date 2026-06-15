## Why

マウスを使わず、キーボードだけでWindows画面上の任意の位置を高速かつ予測可能に指定したい。既存のMouselessは階層的な座標選択を提供するが、全画面グリッドが見づらい・サブグリッドが小さい・不要なオーバーレイが残る・対象が隠れる・クリックに別キーが要る・モードが多い・アクセシビリティAPIの取得可否で挙動が変わる、といった課題がある。これらを解消した、座標ベースのみで動作するOSSツール（Windows 11 / Go）を新規に構築する。

## What Changes

- グローバルショートカット（既定 `Alt+Space`）で起動し、対象モニターをキャプチャして枠なし・最前面オーバーレイを表示する。
- 画面を内部的に規則分割し、各領域中心に1文字ラベルのみを描画する（グリッド線・十字線は表示しない）。
- 通常ラベル入力でその領域へ絞り込み・自動拡大、Shift付きラベル入力で即クリックする。任意の段階でクリック可能（固定段階数を要求しない）。
- `Backspace` で一段戻る、`Esc` で安全にキャンセルする。
- クリック種別はセッション開始時のホットキーで決定する（`Alt+Space`=左、`Alt+R`=右、`Alt+D`=ダブル）。選択操作はクリック種別に依らず統一する。
- 初回キャプチャ画像の切り出し・拡大のみを使い、段階ごとに再キャプチャしない（決定論的・低遅延）。
- Per-Monitor DPI Aware で動作し、Full HD / WQHD / 4K で座標ずれを起こさない。
- タスクトレイに常駐し、GUIで主要設定（ホットキー、ラベル数/サイズ、最大段階数、アニメーション、自動起動）を変更できる。Windowsログイン時に自動起動できる。
- UI Automation・OCR・画像認識・AIによる対象予測・Free Mode・複数モード切替は採用しない。キャプチャ画像・入力キー・座標を外部送信せず、ファイル保存もしない。

## Capabilities

### New Capabilities
- `session-lifecycle`: セッションの起動・多重起動防止・モニターキャプチャ取得と破棄・状態遷移（Idle→Capturing→Selecting→Zooming/Executing→Idle）・エラー時の安全な復帰。
- `keyboard-input`: グローバルホットキー登録と、選択中の入力を文字結果ではなく仮想キーコード+Shift状態として捕捉（IME/Caps Lock非依存）。
- `spatial-selection`: 検索領域の規則分割・Anchor生成・通常入力による絞り込み・Backspaceによる段階復帰・オーバーレイ座標から物理画面座標への変換・決定論性の保証。
- `overlay-rendering`: 対象モニター全体を覆う枠なし最前面オーバーレイ、ラベルのみの半透明描画（グリッド線非表示、明暗どちらの背景でも可読）、選択後の不要ラベル即時消去、Per-Monitor DPI対応。
- `click-execution`: 左/右/ダブルクリックの実行、座標へのカーソル移動、実行順序（オーバーレイ非表示→カーソル移動→入力送信→セッション終了）。
- `settings-config`: `config.json` の読み書き、GUI設定画面、初期設定なしでの即利用、破損時の既定値起動と退避。
- `tray-autostart`: タスクトレイ常駐、Windowsログイン時自動起動、初回チュートリアル。

### Modified Capabilities
（既存仕様なし。新規プロジェクトのため変更対象はない。）

## Impact

- 新規Goプロジェクト（Windows専用）。パッケージ構成: `cmd/app`, `internal/{app,session,spatial,overlay,capture,input,monitor,settings,tray,tutorial,win32}`。
- Win32 API依存（RegisterHotKey/低レベルキーフック、画面キャプチャ、レイヤードウィンドウ、SendInput、DPI Awareness）。Win32呼び出しは `internal/win32` に隔離する。
- ライセンス MIT、配布形態 OSS。テレメトリ・ネットワーク送信なし。
- MVP範囲は単一モニター・左クリックをMust、右/ダブルクリック・チュートリアル・ラベルサイズ/アニメーション・状態表示をShouldとする。複数モニター・ドラッグ・winget配布等はCould以降。
