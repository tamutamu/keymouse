param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v\d+\.\d+\.\d+$')]
    [string]$Version
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$dist = Join-Path $root "dist"
$staging = Join-Path $dist "keymouse-$Version-windows-amd64"
$zipPath = Join-Path $dist "keymouse-$Version-windows-amd64.zip"
$sumsPath = Join-Path $dist "SHA256SUMS.txt"

Remove-Item -LiteralPath $staging -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $staging | Out-Null

Push-Location $root
try {
    & .\build.ps1

    Copy-Item -LiteralPath (Join-Path $root "keymouse.exe") -Destination $staging
    Copy-Item -LiteralPath (Join-Path $root "README.md") -Destination $staging
    Copy-Item -LiteralPath (Join-Path $root "README_ja.md") -Destination $staging
    Copy-Item -LiteralPath (Join-Path $root "LICENSE") -Destination $staging

    Remove-Item -LiteralPath $zipPath -Force -ErrorAction SilentlyContinue
    Compress-Archive -Path (Join-Path $staging "*") -DestinationPath $zipPath

    $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath
    "$($hash.Hash.ToLowerInvariant())  $(Split-Path -Leaf $zipPath)" | Set-Content -Encoding ASCII -Path $sumsPath
}
finally {
    Pop-Location
}

Write-Host "Created $zipPath"
Write-Host "Created $sumsPath"
