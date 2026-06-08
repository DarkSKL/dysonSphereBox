@echo off
setlocal EnableExtensions

set "APP_NAME=DysonSphereBox"
set "ROOT_DIR=%~dp0"
set "BIN_DIR=%ROOT_DIR%build\bin"
set "RELEASE_DIR=%ROOT_DIR%dist\release"
set "STAGE_DIR=%ROOT_DIR%dist\stage"
set "EXE_PATH=%BIN_DIR%\%APP_NAME%.exe"
set "EXE_PATH_NO_EXT=%BIN_DIR%\%APP_NAME%"
set "STAGE_EXE_PATH=%STAGE_DIR%\%APP_NAME%.exe"
set "AMD64_ZIP=%RELEASE_DIR%\%APP_NAME%_windows_amd64.zip"
set "ARM64_ZIP=%RELEASE_DIR%\%APP_NAME%_windows_arm64.zip"
set "CHECKSUM_FILE=%RELEASE_DIR%\checksums.txt"

cd /d "%ROOT_DIR%"
if errorlevel 1 exit /b 1

where wails >nul 2>nul
if errorlevel 1 (
  echo [ERROR] wails command not found. Please install Wails CLI first.
  exit /b 1
)

where powershell >nul 2>nul
if errorlevel 1 (
  echo [ERROR] powershell command not found. It is required to create zip files and checksums.
  exit /b 1
)

if not exist "%RELEASE_DIR%" (
  mkdir "%RELEASE_DIR%"
  if errorlevel 1 exit /b 1
)
if not exist "%STAGE_DIR%" (
  mkdir "%STAGE_DIR%"
  if errorlevel 1 exit /b 1
)

echo [INFO] Cleaning previous release packages...
if exist "%AMD64_ZIP%" del /f /q "%AMD64_ZIP%"
if exist "%ARM64_ZIP%" del /f /q "%ARM64_ZIP%"
if exist "%CHECKSUM_FILE%" del /f /q "%CHECKSUM_FILE%"
if exist "%STAGE_EXE_PATH%" del /f /q "%STAGE_EXE_PATH%"

call :BuildAndZip "windows/amd64" "%AMD64_ZIP%"
if errorlevel 1 exit /b 1

call :BuildAndZip "windows/arm64" "%ARM64_ZIP%"
if errorlevel 1 exit /b 1

echo [INFO] Generating checksums.txt...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$files = @('%AMD64_ZIP%', '%ARM64_ZIP%');" ^
  "$sha = [System.Security.Cryptography.SHA256]::Create();" ^
  "$lines = foreach ($file in $files) { $stream = [System.IO.File]::OpenRead($file); try { $bytes = $sha.ComputeHash($stream) } finally { $stream.Dispose() }; $hash = -join ($bytes | ForEach-Object { $_.ToString('x2') }); '{0}  {1}' -f $hash, [System.IO.Path]::GetFileName($file) };" ^
  "$sha.Dispose();" ^
  "[System.IO.File]::WriteAllLines('%CHECKSUM_FILE%', $lines)"
if errorlevel 1 (
  echo [ERROR] Failed to generate checksums.txt.
  exit /b 1
)

echo.
echo [OK] Release packages created:
echo   %AMD64_ZIP%
echo   %ARM64_ZIP%
echo   %CHECKSUM_FILE%
exit /b 0

:BuildAndZip
set "TARGET_PLATFORM=%~1"
set "ZIP_PATH=%~2"

echo.
echo [INFO] Building %APP_NAME% for %TARGET_PLATFORM%...
wails build -clean -platform "%TARGET_PLATFORM%" -o "%APP_NAME%"
if errorlevel 1 (
  echo [ERROR] Wails build failed for %TARGET_PLATFORM%.
  exit /b 1
)

if exist "%EXE_PATH%" (
  copy /y "%EXE_PATH%" "%STAGE_EXE_PATH%" >nul
) else if exist "%EXE_PATH_NO_EXT%" (
  copy /y "%EXE_PATH_NO_EXT%" "%STAGE_EXE_PATH%" >nul
) else (
  echo [ERROR] Build output not found: %EXE_PATH% or %EXE_PATH_NO_EXT%
  exit /b 1
)
if errorlevel 1 (
  echo [ERROR] Failed to stage executable for %TARGET_PLATFORM%.
  exit /b 1
)

echo [INFO] Creating %ZIP_PATH%...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "Compress-Archive -LiteralPath '%STAGE_EXE_PATH%' -DestinationPath '%ZIP_PATH%' -Force"
if errorlevel 1 (
  echo [ERROR] Failed to create package: %ZIP_PATH%
  exit /b 1
)

exit /b 0
