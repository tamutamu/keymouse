# keymouse をビルドする。
# -H windowsgui で GUI サブシステムの実行ファイルにし、起動時に黒いコンソール
# ウィンドウが表示されないようにする(本アプリはトレイ常駐の GUI アプリのため)。
$ErrorActionPreference = "Stop"
$outputDir = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) "dist\keymouse"
New-Item -ItemType Directory -Force $outputDir | Out-Null
go build -ldflags "-H windowsgui" -o (Join-Path $outputDir "keymouse.exe") ./cmd/app
Write-Host "Built $outputDir\keymouse.exe"
