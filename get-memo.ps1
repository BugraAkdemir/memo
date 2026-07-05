# Memo — one-line installer for Windows.
#
#   irm https://download.bugradev.com/get-memo.ps1 | iex
#
# Downloads the Memo Setup installer and launches it. The installer itself
# (installer.iss, compiled with Inno Setup) does all the real work — data
# dirs, config seeding, desktop/start-menu icons, PATH — this script's only
# job is fetch + run, exactly like double-clicking a downloaded Setup.exe.
#
# ASSUMPTION: memo.exe on the domain is the full compiled Inno Setup
# installer (its size on R2, ~600MB, matches that — a plain Go backend
# binary would be a few MB). If it's ever just the bare backend binary
# instead, this script needs to change to the same download+wrapper
# approach get-memo.sh uses for Linux/macOS.

$ErrorActionPreference = "Stop"

Clear-Host

$domain = "https://download.bugradev.com"
$url = "$domain/memo.exe"
$dest = Join-Path $env:TEMP "Memo-Setup.exe"

Write-Host "Downloading: $url"

$ProgressPreference = "Continue"
try {
    Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
} catch {
    Write-Host "Download failed: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Launching installer..."

$proc = Start-Process -FilePath $dest -PassThru
$proc.WaitForExit()

Remove-Item -Path $dest -ErrorAction SilentlyContinue

if ($proc.ExitCode -eq 0) {
    Write-Host "Installation complete."
} else {
    Write-Host "Installer exited with code: $($proc.ExitCode)" -ForegroundColor Red
}
