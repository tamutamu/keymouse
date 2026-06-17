# keymouse をビルドする。
# -H windowsgui で GUI サブシステムの実行ファイルにし、起動時に黒いコンソール
# ウィンドウが表示されないようにする(本アプリはトレイ常駐の GUI アプリのため)。
$ErrorActionPreference = "Stop"
go build -ldflags "-H windowsgui" -o keymouse.exe ./cmd/app
Write-Host "Built keymouse.exe (windowsgui subsystem, no console)"
