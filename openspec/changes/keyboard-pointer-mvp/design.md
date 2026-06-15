## Context

新規Goプロジェクト（Windows 11専用、OSS/MIT）。Mouselessの階層的座標選択を土台に、グリッド非表示・自動拡大・任意段階での即クリックを実現する。実装はWin32 APIへ直接依存する（グローバルホットキー、画面キャプチャ、レイヤードウィンドウ、`SendInput`、Per-Monitor DPI Awareness）。クロスプラットフォーム抽象化は行わず、Win32呼び出しを `internal/win32` に隔離する。アクセシビリティAPI・OCR・画像認識・AIは一切使用しない。決定論性（同一入力→同一座標）と低遅延（起動→初期ラベル p95 100ms、絞り込み→次ラベル p95 50ms）が中核要件。

パッケージ構成: `cmd/app`, `internal/{app,session,spatial,overlay,capture,input,monitor,settings,tray,tutorial,win32}`。

## Goals / Non-Goals

**Goals:**
- 座標ベースのみで、画面に見える任意位置をキーボードで指定・クリックする。
- 1つの操作体系に統一（通常=絞り込み、Shift付き=クリック、Backspace=戻る、Esc=中止）。モードを持たない。
- セッション中は初回キャプチャの切り出し・拡大のみで再キャプチャせず、決定論的かつ低遅延に動作する。
- Per-Monitor DPI Aware で Full HD/WQHD/4K の座標ずれを排除する。
- ローカル完結（ネットワーク送信・テレメトリなし、画像のファイル保存なし）。

**Non-Goals:**
- Free Mode（連続移動）、UI Automation、OCR、画像認識、AI対象予測。
- 複数モニター、ドラッグ&ドロップ、修飾キー付きクリック、winget配布、自動アップデート、CLI/IPC、エージェント向けAPI（MVP対象外、Could/Won't）。
- macOS / Linux 対応。

## Decisions

### D1: グローバルホットキー方式
`RegisterHotKey` を採用する。クリック種別ごとに3つのホットキー（左/右/ダブル）を登録し、押下されたIDでクリック種別を決定する。
- 代替: 低レベルキーフック（`WH_KEYBOARD_LL`）で自前判定 → 常時フック常駐が必要で待機中の負荷とフォーカス干渉リスクが高く、NFR-003（待機中の高頻度ポーリング禁止）に反するため不採用。

### D2: 選択中のキー入力捕捉
セッション開始後はオーバーレイウィンドウへフォーカスを移し、`WM_KEYDOWN` の仮想キーコード(`wParam`)と `GetKeyState(VK_SHIFT)` で判定する。文字メッセージ(`WM_CHAR`)は使わない。これによりIME/Caps Lock非依存を満たす（FR、NFR-034）。

### D3: 空間選択モデル（Anchor）
検索領域 `R` を `cols×rows`（ラベル数25なら5×5）に等分し、各セル中心にAnchorを置く。`Anchor.SourceArea` は次段階の検索領域、`SourcePoint` は元モニター物理座標。すべて `Rect/Point` を `float64` で保持し、丸めは物理座標への最終変換時のみ行う（決定論性確保）。座標変換チェーン: オーバーレイ座標 → 現在検索領域 → 元モニター物理座標。
- ラベルキーは方向の意味を持たせず、固定の英字キー配列から割当（テンキー非依存）。

### D4: 再キャプチャしない拡大
セッション開始時に対象モニター物理ピクセルを1枚キャプチャしメモリ保持。各絞り込みでは `SourceArea` に対応する元画像領域を切り出してオーバーレイへ拡大描画する。画面内容変化による座標ずれを防ぎ、応答速度を上げる（FR-003〜005, NFR-002）。終了時に画像を破棄。

### D5: オーバーレイ実装
枠なし・タイトルバーなし・最前面のレイヤードウィンドウ（`WS_EX_LAYERED | WS_EX_TOPMOST | WS_EX_TOOLWINDOW`）でモニター全体を覆う。WebViewは使わない（D5理由: 起動遅延とDPI/最前面制御の不確実性回避）。半透明背景＋縁取り/ハロー付きラベルで明暗両対応・グリッド線なし。描画はGDI/Direct2Dのいずれか（実装時に遅延要件で選定、まずGDIで計測）。
- プロセスは Per-Monitor v2 DPI Aware（マニフェスト or `SetProcessDpiAwarenessContext`）。

### D6: クリック実行
`SetCursorPos` でカーソル移動後 `SendInput` でクリック送信。実行順序は「オーバーレイ非表示→カーソル移動→入力送信→セッション終了」に固定（自アプリへの誤クリック・オーバーレイ干渉を防ぐ）。ダブルクリックは `GetDoubleClickTime` 以内に2回送信。

### D7: 状態機械
`session` パッケージに明示的ステートマシン（Idle/Capturing/Selecting/Zooming/Executing/Cancelled）を実装。各段階の `SelectionStage`（SearchArea+Anchors）をスタックに積み、Backspaceでpop。エラー/パニックは `recover` で捕捉しオーバーレイ破棄→Idle復帰。

### D8: 設定とトレイ
設定は `%APPDATA%\<ProjectName>\config.json`。破損時は退避(`config.json.bak`等)し既定値起動。トレイ常駐とGUI設定は軽量ライブラリ（例: `getlantern/systray` 系）またはWin32直叩きで実装、依存は最小化。自動起動はレジストリ `Run` キー登録で実現。

## Risks / Trade-offs

- [拡大画像の見え方が実画面とずれる（クリック時点で背後UIが変化）] → クリックは座標へ送るため最終位置は正しいが、視覚は古い。MVPは静止画前提で許容し、ドキュメントに明記。
- [`SendInput` のクリックがDPI仮想化/座標系差で1〜数pxずれる] → 物理座標で送り、Per-Monitor v2に統一して検証（4Kで受け入れ基準12を満たすこと）。
- [`RegisterHotKey` が他アプリと競合し登録失敗] → エラー時に設定画面を開き変更を促す（FR-001/エラー表）。既定の `Alt+Space` は競合可能性があるため代替候補を提示。
- [GDI描画が p95 100/50ms を満たせない] → まずGDIで計測し、未達ならDirect2D/事前合成へ切替。再キャプチャ無し(D4)が遅延の主要対策。
- [トレイ/GUIライブラリ依存の肥大化] → Win32直叩きへフォールバック可能な薄いラッパに留める。

## Open Questions

- プロジェクト名（パッケージ名・APPDATAフォルダ名・レジストリ値に影響）。`未定` のため確定が必要。
- 描画バックエンドはGDIで足りるか、Direct2Dが必要か（性能計測で確定）。
- 25個のラベルに割り当てる具体的英字キー配列と、26+キー必要時のフォールバック規則。
- マルチモニター時の「対象モニター」決定規則（MVPは単一モニター=カーソルのあるモニターを想定）。
