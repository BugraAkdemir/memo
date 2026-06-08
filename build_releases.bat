@echo off
setlocal enabledelayedexpansion

set APP_NAME=Memo
set APP_EXEC=memo_flutter

:: Read version
if exist version (
    set /p VERSION=<version
    set VERSION=!VERSION:V=!
    set VERSION=!VERSION:v=!
    set VERSION=!VERSION: =!
) else (
    set VERSION=3.0.0
)

echo ==========================================================
echo !APP_NAME! v!VERSION! Windows Paketleme
echo ==========================================================
echo.

set STAGEDIR=%cd%\build_output\stage\%APP_NAME%
set DISTDIR=%cd%\build_output\dist

rmdir /s /q "%cd%\build_output" 2>nul
mkdir "%STAGEDIR%\data"
mkdir "%STAGEDIR%\config"
mkdir "%DISTDIR%"

:: 1. Build Go Backend
echo [1/3] Go Backend derleniyor...
set CGO_ENABLED=1
go build -o "%STAGEDIR%\memo-backend.exe" .
if %ERRORLEVEL% neq 0 (
    echo HATA: Go build basarisiz oldu!
    exit /b 1
)

:: 2. Build Flutter Frontend
echo [2/3] Flutter Frontend derleniyor...
cd frontend
call flutter build windows --release
if %ERRORLEVEL% neq 0 (
    echo HATA: Flutter build basarisiz oldu!
    cd ..
    exit /b 1
)
cd ..

xcopy /E /I /Y "frontend\build\windows\x64\release\runner\Release\*" "%STAGEDIR%\" >nul

:: 3. Copy Assets
echo [3/3] Dosyalar kopyalaniyor...

:: Binaries (llama-server + vec0)
mkdir "%STAGEDIR%\binaries"
xcopy /E /I /Y "binaries\*" "%STAGEDIR%\binaries\" >nul 2>&1

:: Check vec0.dll
if not exist "%STAGEDIR%\binaries\windows\vec0.dll" (
    if not exist "%STAGEDIR%\binaries\windows\cpu\vec0.dll" (
        echo UYARI: vec0.dll bulunamadi! Vektor arama calismayacak.
    )
)

:: Config
xcopy /E /I /Y "config\*" "%STAGEDIR%\config\" >nul 2>&1

:: .env
if exist .env.example (
    copy /Y .env.example "%STAGEDIR%\.env" >nul
)

:: Provider & Orchestra examples
if exist data\providers.example.json (
    copy /Y data\providers.example.json "%STAGEDIR%\data\providers.example.json" >nul
)
if exist data\orchestra.json (
    copy /Y data\orchestra.json "%STAGEDIR%\data\orchestra.json" >nul
)

:: Empty data dirs
echo [] > "%STAGEDIR%\data\permissions.json"
mkdir "%STAGEDIR%\data\models" 2>nul
mkdir "%STAGEDIR%\data\memory" 2>nul
mkdir "%STAGEDIR%\data\sessions" 2>nul
mkdir "%STAGEDIR%\data\agent-backups" 2>nul

:: Create runner batch
copy NUL "%STAGEDIR%\run_memo.bat" >nul
(
echo @echo off
echo cd /d "%%~dp0"
echo set PATH=%%~dp0data\bin;%%PATH%%
echo.
echo REM First-run: copy example configs
echo if not exist "%%USERPROFILE%%\\.memo\\data\\providers.json" (
echo     if exist "%%~dp0data\\providers.example.json" (
echo         mkdir "%%USERPROFILE%%\\.memo\\data" 2^>nul
echo         copy "%%~dp0data\\providers.example.json" "%%USERPROFILE%%\\.memo\\data\\providers.json" ^>nul
echo     ^)
echo ^)
echo.
echo taskkill /F /IM memo-backend.exe ^>nul 2^>^&1
echo taskkill /F /IM llama-server.exe ^>nul 2^>^&1
echo.
echo start "" /B memo-backend.exe
echo timeout /t 1 /nobreak ^>nul
echo start "" /WAIT memo_flutter.exe
echo.
echo taskkill /F /IM memo-backend.exe ^>nul 2^>^&1
echo taskkill /F /IM llama-server.exe ^>nul 2^>^&1
) > "%STAGEDIR%\run_memo.bat"

:: Create installer with Inno Setup if available
where iscc >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo.
    echo [4] Inno Setup installer olusturuluyor...
    iscc "installer.iss" /Q
    echo.
    echo Installer: %DISTDIR%\%APP_NAME%-Setup-v%VERSION%.exe
) else (
    echo.
    echo [4] Inno Setup bulunamadi, ZIP paketi olusturuluyor...
    echo     Inno Setup: https://jrsoftware.org/isinfo.php
    powershell -Command "Compress-Archive -Path '%STAGEDIR%\*' -DestinationPath '%DISTDIR%\%APP_NAME%-windows-x64-v%VERSION%.zip' -Force"
    echo.
    echo ZIP: %DISTDIR%\%APP_NAME%-windows-x64-v%VERSION%.zip
)

echo.
echo ==========================================================
echo PAKETLEME TAMAMLANDI!
echo Ciktilar: %DISTDIR%\
echo ==========================================================
dir "%DISTDIR%"

endlocal
