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

:: Binaries (llama-server + vec0) — only copy windows/
mkdir "%STAGEDIR%\binaries"
xcopy /E /I /Y "binaries\windows" "%STAGEDIR%\binaries\windows\" >nul 2>&1

:: Validate binaries
if not exist "%STAGEDIR%\binaries\windows\vec0.dll" (
    if not exist "%STAGEDIR%\binaries\windows\cpu\vec0.dll" (
        echo UYARI: vec0.dll bulunamadi! Vektor arama calismayacak.
    )
)

if not exist "%STAGEDIR%\binaries\windows\cpu\llama-server.exe" (
    if not exist "%STAGEDIR%\binaries\windows\nvidia\llama-server.exe" (
        if not exist "%STAGEDIR%\binaries\windows\amd\llama-server.exe" (
            echo UYARI: llama-server.exe bulunamadi! Motor calismayacak.
        )
    )
)

:: Config — ship ONLY the clean example as config.yaml (never the real one with secrets)
copy /Y "config\config.yaml.example" "%STAGEDIR%\config\config.yaml" >nul 2>&1
copy /Y "config\config.yaml.example" "%STAGEDIR%\config\config.yaml.example" >nul 2>&1

:: .env
if exist .env.example (
    copy /Y .env.example "%STAGEDIR%\.env" >nul
)

:: Provider & Orchestra examples
if exist data\providers.example.json (
    copy /Y data\providers.example.json "%STAGEDIR%\data\providers.example.json" >nul
)
:: Orchestra config is NOT bundled — the app generates clean defaults on first
:: run, so we never ship the developer's personal orchestra setup.

:: Empty data dirs
echo [] > "%STAGEDIR%\data\permissions.json"
mkdir "%STAGEDIR%\data\models" 2>nul
mkdir "%STAGEDIR%\data\memory" 2>nul
mkdir "%STAGEDIR%\data\sessions" 2>nul
mkdir "%STAGEDIR%\data\agent-backups" 2>nul
mkdir "%STAGEDIR%\data\skills" 2>nul

:: Create runner batch
copy NUL "%STAGEDIR%\run_memo.bat" >nul
(
echo @echo off
setlocal enabledelayedexpansion
echo cd /d "%%~dp0"
echo.
echo set "APP_DIR=%%~dp0"
echo set "MEMO_HOME=%%USERPROFILE%%\\.memo"
echo :: Pin the app's data directory to this writable workspace
echo set "MEMO_DATA_DIR=%%MEMO_HOME%%\data"
echo.
echo :: Writable workspace
echo if not exist "%%MEMO_HOME%%\data\bin" mkdir "%%MEMO_HOME%%\data\bin"
echo if not exist "%%MEMO_HOME%%\data\models" mkdir "%%MEMO_HOME%%\data\models"
echo if not exist "%%MEMO_HOME%%\data\memory" mkdir "%%MEMO_HOME%%\data\memory"
echo if not exist "%%MEMO_HOME%%\data\sessions" mkdir "%%MEMO_HOME%%\data\sessions"
echo if not exist "%%MEMO_HOME%%\data\agent-backups" mkdir "%%MEMO_HOME%%\data\agent-backups"
echo if not exist "%%MEMO_HOME%%\data\skills" mkdir "%%MEMO_HOME%%\data\skills"
echo if not exist "%%MEMO_HOME%%\config" mkdir "%%MEMO_HOME%%\config"
echo.
echo :: First-run: copy bundled binaries to writable location
echo if not exist "%%MEMO_HOME%%\binaries" (
echo     if exist "%%APP_DIR%%binaries" (
echo         echo Ilk calistirma: engine binary'leri kopyalaniyor...
echo         mkdir "%%MEMO_HOME%%\binaries"
echo         xcopy /E /I /Y "%%APP_DIR%%binaries" "%%MEMO_HOME%%\binaries\" ^>nul
echo     ^)
echo ^)
echo.
echo :: First-run: copy config
echo if not exist "%%MEMO_HOME%%\config\config.yaml" (
echo     if exist "%%APP_DIR%%config" (
echo         xcopy /E /I /Y "%%APP_DIR%%config" "%%MEMO_HOME%%\config\" ^>nul
echo     ^)
echo ^)
echo.
echo :: First-run: copy example providers
echo if not exist "%%MEMO_HOME%%\data\providers.json" (
echo     if exist "%%APP_DIR%%data\providers.example.json" (
echo         copy "%%APP_DIR%%data\providers.example.json" "%%MEMO_HOME%%\data\providers.json" ^>nul
echo     ^)
echo ^)
echo.
echo :: First-run: copy orchestra config
echo if not exist "%%MEMO_HOME%%\data\orchestra.json" (
echo     if exist "%%APP_DIR%%data\orchestra.json" (
echo         copy "%%APP_DIR%%data\orchestra.json" "%%MEMO_HOME%%\data\orchestra.json" ^>nul
echo     ^)
echo ^)
echo.
echo :: First-run: .env
echo if not exist "%%MEMO_HOME%%\\.env" (
echo     if exist "%%APP_DIR%%.env" (
echo         copy "%%APP_DIR%%.env" "%%MEMO_HOME%%\\.env" ^>nul
echo     ^)
echo ^)
echo.
echo :: First-run: empty permissions
echo if not exist "%%MEMO_HOME%%\data\permissions.json" (
echo     echo [] ^> "%%MEMO_HOME%%\data\permissions.json"
echo ^)
echo.
echo :: Set PATH to include bundled binary directories
echo set "PATH=%%APP_DIR%%binaries\windows;%%APP_DIR%%binaries\windows\cpu;%%APP_DIR%%binaries\windows\nvidia;%%APP_DIR%%binaries\windows\amd;%%MEMO_HOME%%\data\bin;%%PATH%%"
echo.
echo :: Stop old processes
echo taskkill /F /IM memo-backend.exe ^>nul 2^>^&1
echo taskkill /F /IM llama-server.exe ^>nul 2^>^&1
echo timeout /t 1 /nobreak ^>nul
echo.
echo :: Start backend from writable directory
echo cd /d "%%MEMO_HOME%%"
echo start "" /B "%%APP_DIR%%memo-backend.exe"
echo timeout /t 2 /nobreak ^>nul
echo.
echo :: Start Flutter frontend
echo cd /d "%%APP_DIR%%"
echo start "" /WAIT %APP_EXEC%.exe
echo.
echo :: Cleanup
echo taskkill /F /IM memo-backend.exe ^>nul 2^>^&1
echo taskkill /F /IM llama-server.exe ^>nul 2^>^&1
echo endlocal
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
