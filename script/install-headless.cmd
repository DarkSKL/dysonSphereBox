@echo off
setlocal EnableExtensions
cd /d "%~dp0"
title DSP Nebula Headless Console - Author DarkSKL
color 0B

if /i "%~1"=="--help" goto :help
if /i "%~1"=="/?" goto :help

set "HEADLESS_MODE="
if /i "%~1"=="--check" (
  set "HEADLESS_MODE=-Mode Check"
  shift
)
if /i "%~1"=="--install" (
  set "HEADLESS_MODE=-Mode Install"
  shift
)
if /i "%~1"=="--repair" (
  set "HEADLESS_MODE=-Mode Repair"
  shift
)
if /i "%~1"=="--start" (
  set "HEADLESS_MODE=-Mode Start"
  shift
)
if /i "%~1"=="--start-wait" (
  set "HEADLESS_MODE=-Mode StartWait"
  shift
)
if /i "%~1"=="--stop" (
  set "HEADLESS_MODE=-Mode Stop"
  shift
)
if /i "%~1"=="--logs" (
  set "HEADLESS_MODE=-Mode Logs"
  shift
)

set "PS_EXE="
set "GUIDE_FILE=%~dp0PowerShell-Missing-Guide.txt"

if not defined PS_EXE call :try_path "%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"
if not defined PS_EXE call :try_path "%SystemRoot%\SysWOW64\WindowsPowerShell\v1.0\powershell.exe"
if not defined PS_EXE call :try_path "%ProgramFiles%\PowerShell\7\pwsh.exe"
if not defined PS_EXE call :try_path "%ProgramFiles%\PowerShell\6\pwsh.exe"
if not defined PS_EXE call :try_path "%ProgramFiles(x86)%\PowerShell\7\pwsh.exe"
if not defined PS_EXE call :try_path "%ProgramFiles(x86)%\PowerShell\6\pwsh.exe"
if not defined PS_EXE call :try_path "%LOCALAPPDATA%\Microsoft\WindowsApps\pwsh.exe"

if not defined PS_EXE call :try_reg "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\powershell.exe"
if not defined PS_EXE call :try_reg "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\pwsh.exe"
if not defined PS_EXE call :try_reg "HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\powershell.exe"
if not defined PS_EXE call :try_reg "HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\pwsh.exe"
if not defined PS_EXE call :try_reg "HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\powershell.exe"
if not defined PS_EXE call :try_reg "HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\pwsh.exe"

if not defined PS_EXE call :try_where powershell.exe
if not defined PS_EXE call :try_where pwsh.exe

if not defined PS_EXE (
  call :write_missing_guide
  echo [ERROR] PowerShell not found.
  echo [ERROR] Checked Windows PowerShell 5.x, PowerShell 6/7, App Paths, PATH and WindowsApps.
  echo [ERROR] Guide written to:
  echo         %GUIDE_FILE%
  echo [ERROR] Recommended next steps:
  echo         1. On Windows 10/11, enable Windows PowerShell 5.1 or install PowerShell 7.
  echo         2. If winget exists, run: winget install Microsoft.PowerShell
  pause
  exit /b 1
)

if not exist "%~dp0install-headless.ps1" (
  echo [ERROR] install-headless.ps1 not found.
  echo [ERROR] Keep install-headless.cmd and install-headless.ps1 in the same folder.
  pause
  exit /b 1
)

call :banner
echo [INFO] PowerShell: %PS_EXE%
echo [INFO] Launching headless environment console...
"%PS_EXE%" -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-headless.ps1" %HEADLESS_MODE% %*
set "EXIT_CODE=%ERRORLEVEL%"

if not "%EXIT_CODE%"=="0" (
  echo.
  echo [ERROR] Installer failed with exit code %EXIT_CODE%.
  pause
)

endlocal & exit /b %EXIT_CODE%

:banner
echo.
echo ============================================================
echo   DSP Nebula Headless Console
echo   Author: DarkSKL    WeChat: DarkSKL
echo ============================================================
echo.
goto :eof

:try_path
if defined PS_EXE goto :eof
if not "%~1"=="" if exist "%~1" set "PS_EXE=%~1"
goto :eof

:try_where
if defined PS_EXE goto :eof
for /f "delims=" %%I in ('where %~1 2^>nul') do (
  if not defined PS_EXE if exist "%%~I" set "PS_EXE=%%~I"
)
goto :eof

:try_reg
if defined PS_EXE goto :eof
for /f "tokens=2,*" %%A in ('reg query %~1 /ve 2^>nul ^| findstr /R /C:"REG_"') do (
  if not defined PS_EXE if exist "%%B" set "PS_EXE=%%B"
)
goto :eof

:write_missing_guide
> "%GUIDE_FILE%" (
  echo PowerShell missing for DSP Nebula installer
  echo.
  echo Checked paths:
  echo   %SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe
  echo   %SystemRoot%\SysWOW64\WindowsPowerShell\v1.0\powershell.exe
  echo   %ProgramFiles%\PowerShell\7\pwsh.exe
  echo   %ProgramFiles%\PowerShell\6\pwsh.exe
  echo   %ProgramFiles(x86)%\PowerShell\7\pwsh.exe
  echo   %ProgramFiles(x86)%\PowerShell\6\pwsh.exe
  echo   %LOCALAPPDATA%\Microsoft\WindowsApps\pwsh.exe
  echo.
  echo Checked registry App Paths:
  echo   HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\powershell.exe
  echo   HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\pwsh.exe
  echo   HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\powershell.exe
  echo   HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\pwsh.exe
  echo   HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\powershell.exe
  echo   HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\pwsh.exe
  echo.
  echo Recommended fixes:
  echo   1. Windows 10/11:
  echo      - Open Start and search "Windows PowerShell"
  echo      - Or enable legacy Windows PowerShell 5.1 if it was removed
  echo   2. PowerShell 7:
  echo      - If winget exists: winget install Microsoft.PowerShell
  echo      - Or install manually from https://github.com/PowerShell/PowerShell/releases
  echo   3. After installation, reopen cmd and rerun install-headless.cmd
  echo.
  where winget.exe >nul 2>nul
)
where winget.exe >nul 2>nul
if not errorlevel 1 (
  >> "%GUIDE_FILE%" echo   4. winget detected on this machine, so the fastest fix is likely:
  >> "%GUIDE_FILE%" echo      winget install Microsoft.PowerShell
)
goto :eof

:help
call :banner
echo Usage:
echo   install-headless.cmd
echo   install-headless.cmd --check
echo   install-headless.cmd --install -SaveName Temp
echo   install-headless.cmd --repair
echo   install-headless.cmd --start
echo   install-headless.cmd --start-wait
echo   install-headless.cmd --stop
echo   install-headless.cmd --logs
echo   install-headless.cmd -Mode Install -ForceReinstall -NoDashboard
echo.
echo Notes:
echo   1. Run this from the DSP game root where DSPGAME.exe exists.
echo   2. This wrapper supports Windows 10 and Windows 11 PowerShell layouts.
echo   3. It also checks App Paths, PATH and WindowsApps as fallbacks.
echo   4. If PowerShell is still missing, a local guide file will be generated.
echo   5. Without arguments, this opens an interactive environment console.
echo   6. The console can check, install, repair, generate launch script, start, stop and show logs.
echo   7. This wrapper calls install-headless.ps1 internally.
endlocal & exit /b 0
