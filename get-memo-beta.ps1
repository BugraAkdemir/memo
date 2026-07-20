# Memo BETA — one-line installer for Windows.
#
#   irm https://download.bugradev.com/get-memo-beta.ps1 | iex
#
# Downloads the Memo Setup installer and launches it. The Inno Setup
# installer handles everything: data dirs, config seeding, desktop/start-menu
# icons, and PATH. This script is just fetch + run.
#
# ASSUMPTION: memo-beta.exe on the domain is the full compiled Inno Setup
# installer (~600 MB). If it's ever just the bare backend binary instead,
# this script needs to switch to the same download+wrapper approach
# get-memo-beta.sh uses for Linux/macOS.

$ErrorActionPreference = "Stop"

Clear-Host

# ── banner ───────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "  __  __ _____ __  __  ___  " -ForegroundColor Cyan
Write-Host " |  \/  | ____|  \/  |/ _ \ " -ForegroundColor Cyan
Write-Host " | |\/| |  _| | |\/| | | | |" -ForegroundColor Cyan
Write-Host " | |  | | |___| |  | | |_| |" -ForegroundColor Cyan
Write-Host " |_|  |_|_____|_|  |_|\___/ " -ForegroundColor Cyan
Write-Host ""
Write-Host "  Local-first, privacy-focused AI assistant " -NoNewline
Write-Host "(BETA)" -ForegroundColor Yellow
Write-Host "  https://memo.bugradev.com/guide" -ForegroundColor Blue
Write-Host ""

# ── download ─────────────────────────────────────────────────────────────────
$domain = "https://download.bugradev.com"
$url = "$domain/memo-beta.exe"
$dest = Join-Path $env:TEMP "Memo-Beta-Setup.exe"

Write-Host "Downloading: $url" -ForegroundColor White
$ProgressPreference = "Continue"
try {
    Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
} catch {
    Write-Host "Download failed: $_" -ForegroundColor Red
    exit 1
}

# ── launch installer ─────────────────────────────────────────────────────────
Write-Host ""
Write-Host "Launching installer..." -ForegroundColor White
$proc = Start-Process -FilePath $dest -PassThru
$proc.WaitForExit()
Remove-Item -Path $dest -ErrorAction SilentlyContinue

# ── done ─────────────────────────────────────────────────────────────────────
Write-Host ""
if ($proc.ExitCode -eq 0) {
    Write-Host "  Installation complete! (BETA)" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Guide: " -NoNewline
    Write-Host "https://memo.bugradev.com/guide" -ForegroundColor Blue
    Write-Host ""
    Write-Host "  Thank you for trying the Memo BETA!" -ForegroundColor White
} else {
    Write-Host "  Installer exited with code: $($proc.ExitCode)" -ForegroundColor Red
}
