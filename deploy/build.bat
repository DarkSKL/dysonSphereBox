@echo off
setlocal DisableDelayedExpansion

rem Build Windows release archives for both supported architectures.
set "ROOT_DIR=%~dp0.."
for %%I in ("%ROOT_DIR%") do set "ROOT_DIR=%%~fI"
set "RAW_VERSION=%~1"

if not "%~2"=="" (
    echo Usage: %~nx0 [vX.Y.Z]
    exit /b 2
)

if not defined RAW_VERSION set /p "RAW_VERSION=Enter release version (for example v1.0.0): "
if not defined RAW_VERSION (
    echo [ERROR] Version cannot be empty.
    exit /b 2
)

set "VERSION_INPUT=%RAW_VERSION%"
powershell -NoProfile -ExecutionPolicy Bypass -Command "$v = $env:VERSION_INPUT; if ($v -notmatch '^v?[0-9]+\.[0-9]+\.[0-9]+$') { exit 1 }"
if errorlevel 1 (
    echo [ERROR] Invalid version. Use vX.Y.Z or X.Y.Z, for example v1.0.0.
    exit /b 2
)

if /i "%RAW_VERSION:~0,1%"=="v" (
    set "VERSION=v%RAW_VERSION:~1%"
) else (
    set "VERSION=v%RAW_VERSION%"
)
set "VERSION_INPUT=%VERSION%"
set "OUTPUT_DIR=%ROOT_DIR%\deploy\release\%VERSION%"
set "WORK_DIR=%TEMP%\DysonSphereBox-release-%RANDOM%-%RANDOM%"

where wails >nul 2>nul
if errorlevel 1 (
    echo [ERROR] Wails CLI was not found. Install Wails v2 and add wails to PATH.
    exit /b 1
)

if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"
if errorlevel 1 (
    echo [ERROR] Could not create release directory: %OUTPUT_DIR%
    exit /b 1
)
mkdir "%WORK_DIR%\amd64" "%WORK_DIR%\arm64"
if errorlevel 1 (
    echo [ERROR] Could not create temporary packaging directory.
    exit /b 1
)

pushd "%ROOT_DIR%"
if errorlevel 1 (
    echo [ERROR] Could not enter project directory: %ROOT_DIR%
    exit /b 1
)

echo.
echo [1/4] Building Windows amd64, version %VERSION% ...
call wails build -clean -platform windows/amd64 -ldflags "-X DysonSphereBox/core.Version=%VERSION%"
if errorlevel 1 goto build_failed
if not exist "build\bin\DysonSphereBox.exe" goto missing_binary
copy /y "build\bin\DysonSphereBox.exe" "%WORK_DIR%\amd64\DysonSphereBox.exe" >nul
if errorlevel 1 goto copy_failed

echo.
echo [2/4] Building Windows arm64, version %VERSION% ...
call wails build -clean -platform windows/arm64 -ldflags "-X DysonSphereBox/core.Version=%VERSION%"
if errorlevel 1 goto build_failed
if not exist "build\bin\DysonSphereBox.exe" goto missing_binary
copy /y "build\bin\DysonSphereBox.exe" "%WORK_DIR%\arm64\DysonSphereBox.exe" >nul
if errorlevel 1 goto copy_failed

echo.
echo [3/4] Creating updater archives ...
set "PACKAGE_DIR=%WORK_DIR%\amd64"
powershell -NoProfile -ExecutionPolicy Bypass -Command "Compress-Archive -LiteralPath (Join-Path $env:PACKAGE_DIR 'DysonSphereBox.exe') -DestinationPath (Join-Path $env:OUTPUT_DIR 'DysonSphereBox_windows_amd64.zip') -Force"
if errorlevel 1 goto archive_failed
set "PACKAGE_DIR=%WORK_DIR%\arm64"
powershell -NoProfile -ExecutionPolicy Bypass -Command "Compress-Archive -LiteralPath (Join-Path $env:PACKAGE_DIR 'DysonSphereBox.exe') -DestinationPath (Join-Path $env:OUTPUT_DIR 'DysonSphereBox_windows_arm64.zip') -Force"
if errorlevel 1 goto archive_failed

echo.
echo [4/4] Writing SHA256 checksums ...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference = 'Stop'; $names = @('DysonSphereBox_windows_amd64.zip', 'DysonSphereBox_windows_arm64.zip'); $sha = [System.Security.Cryptography.SHA256]::Create(); try { $lines = foreach ($name in $names) { $stream = [System.IO.File]::OpenRead((Join-Path $env:OUTPUT_DIR $name)); try { $hash = -join ($sha.ComputeHash($stream) | ForEach-Object { $_.ToString('x2') }) } finally { $stream.Dispose() }; $hash + '  ' + $name }; [System.IO.File]::WriteAllLines((Join-Path $env:OUTPUT_DIR 'checksums.txt'), [string[]]$lines, [System.Text.Encoding]::ASCII) } finally { $sha.Dispose() }"
if errorlevel 1 goto checksum_failed
if not exist "%OUTPUT_DIR%\checksums.txt" goto checksum_failed

popd
rmdir /s /q "%WORK_DIR%"
echo.
echo Release files created in: %OUTPUT_DIR%
echo Upload these three files to the matching Gitee Release:
echo   DysonSphereBox_windows_amd64.zip
echo   DysonSphereBox_windows_arm64.zip
echo   checksums.txt
exit /b 0

:build_failed
echo [ERROR] Wails build failed. Temporary files are in: %WORK_DIR%
popd
exit /b 1

:missing_binary
echo [ERROR] Wails finished without creating build\bin\DysonSphereBox.exe.
goto build_failed

:copy_failed
echo [ERROR] Could not stage build output.
goto build_failed

:archive_failed
echo [ERROR] Archive creation failed. Temporary files are in: %WORK_DIR%
popd
exit /b 1

:checksum_failed
echo [ERROR] Could not write checksums.txt. Release output may be incomplete: %OUTPUT_DIR%
popd
exit /b 1
