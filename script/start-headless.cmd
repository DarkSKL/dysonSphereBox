@echo off
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"
title DSP Nebula Headless Server - Author DarkSKL
color 0B

set "MODE=%~1"
set "SAVE_NAME=Temp"
set "SAVE_DIR=C:\Users\DarkSKL\Documents\Dyson Sphere Program\Save"
set "BEPINEX_LOG=%~dp0BepInEx\LogOutput.log"
set "OUTPUT_LOG=%~dp0output_log.txt"

if /i "%MODE%"=="--help" goto :help
if /i "%MODE%"=="/?" goto :help

call :banner

if not exist "DSPGAME.exe" (
  echo [ERROR] DSPGAME.exe not found.
  pause
  exit /b 1
)

if not exist "BepInEx\plugins\nebula-NebulaMultiplayerMod" (
  echo [WARN] Nebula mod was not found. Run install-headless.cmd and choose check or repair.
)

if exist "%SAVE_DIR%\%SAVE_NAME%.dsv" (
  set "LOAD_ARGS=-batchmode -nographics -nebula-server -load ""%SAVE_NAME%"""
) else (
  set "LOAD_ARGS=-batchmode -nographics -nebula-server -load-latest"
)

if /i "%MODE%"=="background" (
  start "DSP Nebula Headless" "DSPGAME.exe" %LOAD_ARGS%
  exit /b 0
)

echo [INFO] Starting DSP Nebula headless server in this console.
echo [INFO] Use start-headless.cmd background to start and close this window immediately.
echo [INFO] Arguments: %LOAD_ARGS%
echo.
"DSPGAME.exe" %LOAD_ARGS%
set "EXIT_CODE=%ERRORLEVEL%"
echo.
echo [INFO] DSPGAME.exe exited with code %EXIT_CODE%.
pause
exit /b %EXIT_CODE%

:help
call :banner
echo Usage:
echo   start-headless.cmd
echo   start-headless.cmd background
echo.
echo Modes:
echo   default     Start headless server in this foreground console
echo   background  Start headless server in background and close this window
exit /b 0

:banner
echo.
echo ============================================================
echo   DSP Nebula Headless Server
echo   Author: DarkSKL    WeChat: DarkSKL
echo ============================================================
echo.
goto :eof
