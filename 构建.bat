@echo off
:: 自动切换当前路径到 bat 脚本所在的目录，防止双击运行报错
cd /d "%~dp0"

set VERSION=9.9.9

echo =========================================
echo Start build process. Version: %VERSION%
echo =========================================

echo.
echo [1/4] Building frontend...
cd frontend
call npm run build
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Frontend build failed!
    cd ..
    pause
    exit /b %errorlevel%
)
cd ..

echo.
echo [2/4] Compiling Go...
call go build ./...
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Go build failed!
    pause
    exit /b %errorlevel%
)

echo.
echo [3/4] Generating Wails module...
call wails generate module
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Wails generate module failed!
    pause
    exit /b %errorlevel%
)

echo.
echo [4/4] Running deploy script...
call deploy\build.bat %VERSION%
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Deploy build failed!
    pause
    exit /b %errorlevel%
)

echo.
echo =========================================
echo [SUCCESS] All steps completed!
echo =========================================
pause