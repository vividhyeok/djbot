param(
    [switch]$Release
)

$ErrorActionPreference = "Stop"
$ProjectRoot = $PSScriptRoot | Split-Path -Parent
$BackendDir  = Join-Path $ProjectRoot "backend"
$AppDir      = Join-Path $ProjectRoot "app"
$BinDir      = Join-Path $AppDir "src-tauri\binaries"

$Triple = "x86_64-pc-windows-msvc"

Write-Host "=== [1/3] Building Go backend ===" -ForegroundColor Cyan
Push-Location $BackendDir
go build -o "$BinDir\goworker-$Triple.exe" .
if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
Write-Host "OK: backend/goworker.exe" -ForegroundColor Green
Pop-Location

Write-Host "=== [2/3] Building Tauri app ===" -ForegroundColor Cyan
Push-Location $AppDir
if ($Release) {
    npm run tauri build
} else {
    npm run tauri dev
}
Pop-Location

Write-Host "=== Done ===" -ForegroundColor Green
