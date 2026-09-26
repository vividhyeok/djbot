param(
    [switch]$Release
)

$ErrorActionPreference = "Stop"
$ProjectRoot = $PSScriptRoot | Split-Path -Parent
$BackendDir  = Join-Path $ProjectRoot "backend"
$AppDir      = Join-Path $ProjectRoot "app"
$BinDir      = Join-Path $AppDir "src-tauri\binaries"

$Triple = "x86_64-pc-windows-msvc"

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

function Copy-ToolDependency($CommandName, $DestinationName, $StandaloneUrl, $MinimumBytes) {
    $cmd = Get-Command $CommandName -ErrorAction SilentlyContinue
    $destination = Join-Path $BinDir $DestinationName
    if ($cmd -and (Get-Item -LiteralPath $cmd.Source).Length -ge $MinimumBytes) {
        Copy-Item -LiteralPath $cmd.Source -Destination $destination -Force
        return
    }
    if (-not $StandaloneUrl) { throw "A standalone $CommandName executable (at least $MinimumBytes bytes) is required for local packaging." }
    Write-Host "Downloading standalone $CommandName..." -ForegroundColor Yellow
    Invoke-WebRequest $StandaloneUrl -OutFile $destination
}

Write-Host "=== [1/3] Building Go backend ===" -ForegroundColor Cyan
Push-Location $BackendDir
go build -o "$BinDir\goworker-$Triple.exe" .
if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
Write-Host "OK: backend/goworker.exe" -ForegroundColor Green
Pop-Location

Write-Host "=== [2/4] Staging audio dependencies ===" -ForegroundColor Cyan
Copy-ToolDependency "ffmpeg" "ffmpeg.exe" "" 10000000
Copy-ToolDependency "yt-dlp" "yt-dlp.exe" "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe" 5000000

Write-Host "=== [3/4] Building Tauri app ===" -ForegroundColor Cyan
Push-Location $AppDir
if ($Release) {
    npm run tauri build
} else {
    npm run tauri dev
}
Pop-Location

Write-Host "=== [4/4] Done ===" -ForegroundColor Green
