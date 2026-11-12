@echo off
setlocal DisableDelayedExpansion

rem 职责：选择模型压缩档位并构建 Windows 双架构发布包；边界：不清空现有运行目录。
set "ROOT_DIR=%~dp0.."
for %%I in ("%ROOT_DIR%") do set "ROOT_DIR=%%~fI"
set "RAW_VERSION=%~1"
set "MODEL_MODE_INPUT=%~2"

if not "%~3"=="" goto usage
if not defined RAW_VERSION set /p "RAW_VERSION=Enter release version (for example v2.1.2): "
if not defined RAW_VERSION (
    echo [ERROR] Version cannot be empty.
    exit /b 2
)
set "VERSION_INPUT=%RAW_VERSION%"
powershell -NoProfile -Command "$v = $env:VERSION_INPUT; if ($v -notmatch '^v?[0-9]+\.[0-9]+\.[0-9]+$') { exit 1 }"
if errorlevel 1 (
    echo [ERROR] Invalid version. Use vX.Y.Z or X.Y.Z.
    exit /b 2
)
if /i "%RAW_VERSION:~0,1%"=="v" (
    set "VERSION=v%RAW_VERSION:~1%"
) else (
    set "VERSION=v%RAW_VERSION%"
)

if defined MODEL_MODE_INPUT goto validate_mode
echo.
echo Model compression:
echo   1. High quality  / 1024px
echo   2. Balanced      / 512px  [default]
echo   3. Compact       / 256px
set /p "MODEL_MODE_INPUT=Choose mode [2]: "
if not defined MODEL_MODE_INPUT set "MODEL_MODE_INPUT=2"

:validate_mode
powershell -NoProfile -Command "if ($env:MODEL_MODE_INPUT -notmatch '^(?i:1|2|3|quality|balanced|compact)$') { exit 1 }"
if errorlevel 1 (
    echo [ERROR] Invalid mode. Use 1, 2, 3, quality, balanced or compact.
    exit /b 2
)
set "MODEL_COMPRESSION_MODE=%MODEL_MODE_INPUT%"
if "%MODEL_MODE_INPUT%"=="1" set "MODEL_COMPRESSION_MODE=quality"
if "%MODEL_MODE_INPUT%"=="2" set "MODEL_COMPRESSION_MODE=balanced"
if "%MODEL_MODE_INPUT%"=="3" set "MODEL_COMPRESSION_MODE=compact"
if /i "%MODEL_MODE_INPUT%"=="quality" set "MODEL_COMPRESSION_MODE=quality"
if /i "%MODEL_MODE_INPUT%"=="balanced" set "MODEL_COMPRESSION_MODE=balanced"
if /i "%MODEL_MODE_INPUT%"=="compact" set "MODEL_COMPRESSION_MODE=compact"
set "OUTPUT_DIR=%ROOT_DIR%\deploy\release\%VERSION%\%MODEL_COMPRESSION_MODE%"

where wails >nul 2>nul
if errorlevel 1 (
    echo [ERROR] Wails CLI was not found. Install Wails v2 and add wails to PATH.
    exit /b 1
)
pushd "%ROOT_DIR%"
if errorlevel 1 (
    echo [ERROR] Could not enter project directory.
    exit /b 1
)

echo.
echo Version: %VERSION%   Model mode: %MODEL_COMPRESSION_MODE%
echo [1/3] Building Windows amd64 and compressed frontend ...
rem 使用独立输出文件，保留正在运行的默认 EXE、数据库和资源库。
call wails build -m -nosyncgomod -platform windows/amd64 -o DysonSphereBox-release-amd64.exe -ldflags "-X DysonSphereBox/core.Version=%VERSION%"
if errorlevel 1 goto build_failed
if not exist "build\bin\DysonSphereBox-release-amd64.exe" goto missing_binary

echo.
echo [2/3] Building Windows arm64 with the same compressed frontend ...
call wails build -m -nosyncgomod -s -skipbindings -platform windows/arm64 -o DysonSphereBox-release-arm64.exe -ldflags "-X DysonSphereBox/core.Version=%VERSION%"
if errorlevel 1 goto build_failed
if not exist "build\bin\DysonSphereBox-release-arm64.exe" goto missing_binary

echo.
echo [3/3] Packaging EXEs, updater archives, profile report and SHA256 checksums ...
powershell -NoProfile -File "%ROOT_DIR%\deploy\package-release.ps1" -Version "%VERSION%" -Mode "%MODEL_COMPRESSION_MODE%"
if errorlevel 1 goto package_failed
popd
echo.
echo Model mode: %MODEL_COMPRESSION_MODE%
echo Release files: %OUTPUT_DIR%
echo Upload the two ZIP files and checksums.txt to the matching Gitee Release.
exit /b 0

:usage
echo Usage: %~nx0 [vX.Y.Z] [1^|2^|3^|quality^|balanced^|compact]
exit /b 2

:build_failed
echo [ERROR] Wails build failed. Existing release files were preserved.
popd
exit /b 1

:missing_binary
echo [ERROR] Wails finished without creating the expected EXE.
goto build_failed

:package_failed
echo [ERROR] Packaging failed. Build outputs remain in build\bin.
popd
exit /b 1
