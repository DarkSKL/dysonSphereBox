param(
  [ValidateSet('Console', 'Check', 'Install', 'Repair', 'Start', 'StartWait', 'Stop', 'Logs', 'GenerateStartScript')]
  [string]$Mode = 'Console',
  [string]$SaveName = 'Temp',
  [switch]$StartAfterInstall,
  [switch]$ForceReinstall,
  [switch]$NoDashboard
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$script:ToolName = 'DSP Nebula Headless Console'
$script:Author = 'DarkSKL'
$script:Wechat = 'DarkSKL'

try {
  $host.UI.RawUI.WindowTitle = "$script:ToolName - Author $script:Author"
}
catch {
}

if ($Mode -eq 'Console' -and ($NoDashboard -or $StartAfterInstall -or $ForceReinstall)) {
  $Mode = 'Install'
}

function Write-Banner {
  param([string]$Title = $script:ToolName)

  Write-Host ''
  Write-Host '╔══════════════════════════════════════════════════════════════╗' -ForegroundColor DarkCyan
  Write-Host ('║  {0,-58}║' -f $Title) -ForegroundColor Cyan
  Write-Host ('║  作者: {0,-18} 微信: {1,-22}║' -f $script:Author, $script:Wechat) -ForegroundColor Yellow
  Write-Host '╚══════════════════════════════════════════════════════════════╝' -ForegroundColor DarkCyan
  Write-Host ''
}

function Write-Log {
  param(
    [ValidateSet('INFO', 'OK', 'WARN', 'ERROR', 'STEP')]
    [string]$Level,
    [string]$Message
  )

  $prefix = switch ($Level) {
    'INFO' { '[信息]' }
    'OK' { '[完成]' }
    'WARN' { '[警告]' }
    'ERROR' { '[错误]' }
    'STEP' { '[步骤]' }
  }

  $color = switch ($Level) {
    'INFO' { 'Cyan' }
    'OK' { 'Green' }
    'WARN' { 'Yellow' }
    'ERROR' { 'Red' }
    'STEP' { 'Magenta' }
  }

  Write-Host "$prefix $Message" -ForegroundColor $color
}

function Ensure-Directory {
  param([string]$Path)

  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Path $Path | Out-Null
  }
}

function Wait-ForAnyKey {
  param([string]$Text = '按任意键返回控制台...')

  Write-Host ''
  Write-Host $Text -ForegroundColor DarkGray
  [void][System.Console]::ReadKey($true)
}

function Invoke-WithRetry {
  param(
    [scriptblock]$ScriptBlock,
    [int]$Retries = 3,
    [int]$DelaySeconds = 2,
    [string]$ActionName = '操作'
  )

  for ($try = 1; $try -le $Retries; $try++) {
    try {
      return & $ScriptBlock
    }
    catch {
      if ($try -ge $Retries) {
        throw
      }
      Write-Log WARN "$ActionName 失败，$DelaySeconds 秒后重试 ($try/$Retries): $($_.Exception.Message)"
      Start-Sleep -Seconds $DelaySeconds
    }
  }
}

function Test-GameRoot {
  $required = @('DSPGAME.exe', 'DSPGAME_Data', 'MonoBleedingEdge')
  $missing = @()

  foreach ($item in $required) {
    if (-not (Test-Path -LiteralPath (Join-Path $script:GameRoot $item))) {
      $missing += $item
    }
  }

  [PSCustomObject]@{
    Ok = ($missing.Count -eq 0)
    Missing = $missing
  }
}

function Assert-GameRoot {
  $state = Test-GameRoot
  if (-not $state.Ok) {
    throw "当前目录不是戴森球计划游戏根目录，缺少: $($state.Missing -join ', ')"
  }
}

function Test-Internet {
  try {
    Invoke-WebRequest -Uri 'https://thunderstore.io' -UseBasicParsing -Method Head -TimeoutSec 12 | Out-Null
    return $true
  }
  catch {
    return $false
  }
}

function Get-ThunderstorePackages {
  if ($script:PackageIndex) {
    return $script:PackageIndex
  }

  Write-Log STEP '获取 Thunderstore 社区包索引...'
  $url = 'https://thunderstore.io/c/dyson-sphere-program/api/v1/package/'
  $script:PackageIndex = Invoke-WithRetry -ActionName '获取包索引' -ScriptBlock {
    Invoke-RestMethod -Uri $url -TimeoutSec 60
  }
  Write-Log OK "已获取包索引，数量: $($script:PackageIndex.Count)"
  return $script:PackageIndex
}

function Resolve-LatestPackage {
  param(
    [string]$Owner,
    [string]$Name
  )

  $pkg = Get-ThunderstorePackages | Where-Object {
    $_.owner -eq $Owner -and $_.name -eq $Name
  } | Select-Object -First 1

  if (-not $pkg) {
    throw "未找到 Thunderstore 包: $Owner/$Name"
  }

  $version = $pkg.versions | Where-Object { $_.is_active } | Sort-Object {
    [datetime]$_.date_created
  } -Descending | Select-Object -First 1

  if (-not $version) {
    throw "包没有可用版本: $Owner/$Name"
  }

  [PSCustomObject]@{
    Owner = $Owner
    Name = $Name
    FullName = $version.full_name
    Version = $version.version_number
    DownloadUrl = $version.download_url
    Dependencies = @($version.dependencies)
  }
}

function Resolve-RequiredPackages {
  @(
    Resolve-LatestPackage -Owner 'xiaoye97' -Name 'BepInEx'
    Resolve-LatestPackage -Owner 'nebula' -Name 'NebulaMultiplayerMod'
  )
}

function Parse-DependencyString {
  param([string]$Dependency)

  $lastDash = $Dependency.LastIndexOf('-')
  if ($lastDash -lt 1) {
    throw "无法解析依赖格式: $Dependency"
  }

  $left = $Dependency.Substring(0, $lastDash)
  $version = $Dependency.Substring($lastDash + 1)
  $firstDash = $left.IndexOf('-')
  if ($firstDash -lt 1) {
    throw "无法解析依赖格式: $Dependency"
  }

  [PSCustomObject]@{
    Owner = $left.Substring(0, $firstDash)
    Name = $left.Substring($firstDash + 1)
    Version = $version
  }
}

function Resolve-PackageGraph {
  param([string[]]$RootDependencies)

  $resolved = [ordered]@{}
  $resolving = [System.Collections.Generic.HashSet[string]]::new()

  function Add-Dependency {
    param([string]$Dependency)

    if ([string]::IsNullOrWhiteSpace($Dependency)) {
      return
    }

    if ($resolved.Contains($Dependency)) {
      return
    }

    if ($resolving.Contains($Dependency)) {
      Write-Log WARN "检测到循环依赖，跳过重复展开: $Dependency"
      return
    }

    $parsed = Parse-DependencyString -Dependency $Dependency
    [void]$resolving.Add($Dependency)
    Write-Log INFO "解析依赖: $Dependency"

    try {
      $pkg = Resolve-LatestPackage -Owner $parsed.Owner -Name $parsed.Name
      $pkg | Add-Member -NotePropertyName RequestedDependency -NotePropertyValue $Dependency -Force
      foreach ($child in $pkg.Dependencies) {
        Add-Dependency -Dependency $child
      }
      $resolved[$Dependency] = $pkg
    }
    finally {
      [void]$resolving.Remove($Dependency)
    }
  }

  foreach ($dep in $RootDependencies) {
    Add-Dependency -Dependency $dep
  }

  return @($resolved.Values)
}

function Test-ZipFile {
  param([string]$Path)

  if (-not (Test-Path -LiteralPath $Path)) {
    return $false
  }

  try {
    Add-Type -AssemblyName System.IO.Compression.FileSystem -ErrorAction SilentlyContinue
    $zip = [System.IO.Compression.ZipFile]::OpenRead($Path)
    $zip.Dispose()
    return $true
  }
  catch {
    return $false
  }
}

function Download-Package {
  param([pscustomobject]$Package)

  Ensure-Directory -Path $script:DownloadRoot
  $safeName = '{0}-{1}-{2}.zip' -f $Package.Owner, $Package.Name, $Package.Version
  $zipPath = Join-Path $script:DownloadRoot $safeName
  $partialPath = "$zipPath.partial"

  if ((-not $ForceReinstall) -and (Test-ZipFile -Path $zipPath)) {
    Write-Log INFO "复用已校验缓存: $safeName"
    return $zipPath
  }

  if (Test-Path -LiteralPath $zipPath) {
    Write-Log WARN "删除不可用缓存: $safeName"
    Remove-Item -LiteralPath $zipPath -Force
  }
  if (Test-Path -LiteralPath $partialPath) {
    Remove-Item -LiteralPath $partialPath -Force
  }

  Write-Log STEP "下载 $($Package.Owner)/$($Package.Name) $($Package.Version)"
  Invoke-WithRetry -ActionName "下载 $safeName" -ScriptBlock {
    Invoke-WebRequest -Uri $Package.DownloadUrl -OutFile $partialPath -TimeoutSec 180
  } | Out-Null

  Move-Item -LiteralPath $partialPath -Destination $zipPath -Force
  if (-not (Test-ZipFile -Path $zipPath)) {
    Remove-Item -LiteralPath $zipPath -Force -ErrorAction SilentlyContinue
    throw "下载文件不是有效 ZIP: $safeName"
  }

  Write-Log OK "下载完成并通过 ZIP 校验: $safeName"
  return $zipPath
}

function Expand-ZipToTemp {
  param(
    [string]$ZipPath,
    [string]$TempName
  )

  $extractPath = Join-Path $script:WorkRoot $TempName
  if (Test-Path -LiteralPath $extractPath) {
    for ($try = 1; $try -le 5; $try++) {
      try {
        Remove-Item -LiteralPath $extractPath -Recurse -Force
        break
      }
      catch {
        if ($try -eq 5) {
          throw
        }
        Start-Sleep -Milliseconds 400
      }
    }
  }
  Expand-Archive -LiteralPath $ZipPath -DestinationPath $extractPath -Force
  return $extractPath
}

function Backup-ExistingInstall {
  if (-not (Test-Path -LiteralPath $script:BepInExRoot) -and -not (Test-Path -LiteralPath (Join-Path $script:GameRoot 'winhttp.dll'))) {
    return
  }

  $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
  $backupDir = Join-Path $script:GameRoot ("_headless_install_backup_{0}" -f $stamp)
  Ensure-Directory -Path $backupDir

  Write-Log STEP "备份当前模组环境到: $backupDir"
  foreach ($name in @('BepInEx', 'winhttp.dll', 'doorstop_config.ini', 'changelog.txt', '.doorstop_version')) {
    $source = Join-Path $script:GameRoot $name
    if (Test-Path -LiteralPath $source) {
      Copy-Item -LiteralPath $source -Destination (Join-Path $backupDir $name) -Recurse -Force
    }
  }
  Write-Log OK '备份完成'
}

function Install-BepInExPack {
  param([pscustomobject]$Package)

  $zip = Download-Package -Package $Package
  $extract = Expand-ZipToTemp -ZipPath $zip -TempName ('extract-' + $Package.Owner + '-' + $Package.Name + '-' + $Package.Version)
  $packRoot = Join-Path $extract 'BepInExPack'

  if (-not (Test-Path -LiteralPath $packRoot)) {
    throw 'BepInEx 包结构异常，缺少 BepInExPack 目录'
  }

  Write-Log STEP '安装 BepInEx 基础框架...'
  Copy-Item -Path (Join-Path $packRoot '*') -Destination $script:GameRoot -Recurse -Force
  Write-Log OK 'BepInEx 基础框架安装完成'
}

function Install-ModPackage {
  param([pscustomobject]$Package)

  $zip = Download-Package -Package $Package
  $extract = Expand-ZipToTemp -ZipPath $zip -TempName ('extract-' + $Package.Owner + '-' + $Package.Name + '-' + $Package.Version)

  $pluginDirName = '{0}-{1}' -f $Package.Owner, $Package.Name
  $targetDir = Join-Path $script:PluginsRoot $pluginDirName
  Ensure-Directory -Path $targetDir

  Write-Log STEP "安装模组: $pluginDirName"
  Get-ChildItem -LiteralPath $extract -Force | ForEach-Object {
    Copy-Item -LiteralPath $_.FullName -Destination $targetDir -Recurse -Force
  }
  Write-Log OK "模组安装完成: $pluginDirName"
}

function Ensure-SteamAppId {
  Set-Content -LiteralPath (Join-Path $script:GameRoot 'steam_appid.txt') -Value '1366540' -Encoding ascii
  Write-Log OK '已写入 steam_appid.txt'
}

function Ensure-DoorstopConfig {
  $cfgPath = Join-Path $script:GameRoot 'doorstop_config.ini'
  if (-not (Test-Path -LiteralPath $cfgPath)) {
    return
  }

  $content = Get-Content -LiteralPath $cfgPath -Raw
  $content = $content -replace '(?m)^enabled\s*=.*$', 'enabled=true'
  $content = $content -replace '(?m)^targetAssembly\s*=.*$', 'targetAssembly=BepInEx\core\BepInEx.Preloader.dll'
  Set-Content -LiteralPath $cfgPath -Value $content -Encoding ascii
  Write-Log OK '已修复 Doorstop 启用状态'
}

function Ensure-BepInExConsole {
  $cfgPath = Join-Path $script:BepInExConfigRoot 'BepInEx.cfg'
  if (-not (Test-Path -LiteralPath $cfgPath)) {
    return
  }

  $content = Get-Content -LiteralPath $cfgPath -Raw
  $content = $content -replace '(?m)^Enabled = false$', 'Enabled = true'
  $content = $content -replace '(?m)^WriteUnityLog = false$', 'WriteUnityLog = true'
  $encoding = if ($PSVersionTable.PSVersion.Major -ge 6) { 'utf8NoBOM' } else { 'UTF8' }
  Set-Content -LiteralPath $cfgPath -Value $content -Encoding $encoding
  Write-Log OK '已启用 BepInEx 控制台与磁盘日志'
}

function Initialize-BepInExConfig {
  Ensure-Directory -Path $script:BepInExConfigRoot
  if (Test-Path -LiteralPath (Join-Path $script:BepInExConfigRoot 'BepInEx.cfg')) {
    return
  }

  Write-Log STEP '首次生成 BepInEx 配置...'
  $proc = Start-Process -FilePath (Join-Path $script:GameRoot 'DSPGAME.exe') -ArgumentList '-batchmode', '-nographics' -WorkingDirectory $script:GameRoot -PassThru
  Start-Sleep -Seconds 8
  if (-not $proc.HasExited) {
    Stop-Process -Id $proc.Id -Force
  }
}

function Write-StartScript {
  $startCmdPath = Join-Path $script:GameRoot 'start-headless.cmd'
  $saveDir = Join-Path $env:USERPROFILE 'Documents\Dyson Sphere Program\Save'

  $cmd = @"
@echo off
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"
title DSP Nebula Headless Server - Author DarkSKL
color 0B

set "MODE=%~1"
set "SAVE_NAME=$SaveName"
set "SAVE_DIR=$saveDir"
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
"@

  Set-Content -LiteralPath $startCmdPath -Value $cmd -Encoding ascii
  Write-Log OK "已生成启动脚本: $startCmdPath"
}

function Get-ServerProcess {
  Get-Process -Name 'DSPGAME' -ErrorAction SilentlyContinue | Select-Object -First 1
}

function Get-NebulaHostPort {
  $defaultPort = 8469
  $cfgPath = Join-Path $script:BepInExConfigRoot 'nebula.cfg'
  if (-not (Test-Path -LiteralPath $cfgPath)) {
    return $defaultPort
  }

  try {
    $match = Get-Content -LiteralPath $cfgPath | Select-String -Pattern '^\s*HostPort\s*=\s*(\d+)\s*$' | Select-Object -First 1
    if ($match -and $match.Matches[0].Groups[1].Value) {
      return [int]$match.Matches[0].Groups[1].Value
    }
  }
  catch {
    Write-Log WARN "读取 Nebula 端口配置失败，使用默认端口 ${defaultPort}: $($_.Exception.Message)"
  }

  return $defaultPort
}

function Test-PortListening {
  param([int]$Port)

  $tcpListening = $false
  $udpListening = $false

  try {
    $tcpListening = [bool](Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1)
  }
  catch {
    $tcpListening = [bool](netstat -ano -p tcp | Select-String -Pattern (":$Port\s+.*LISTENING"))
  }

  try {
    $udpListening = [bool](Get-NetUDPEndpoint -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -First 1)
  }
  catch {
    $udpListening = [bool](netstat -ano -p udp | Select-String -Pattern (":$Port\s+"))
  }

  [PSCustomObject]@{
    Port = $Port
    Tcp = $tcpListening
    Udp = $udpListening
    Any = ($tcpListening -or $udpListening)
  }
}

function Start-HeadlessServer {
  param([switch]$Wait)

  $startCmdPath = Join-Path $script:GameRoot 'start-headless.cmd'
  if (-not (Test-Path -LiteralPath $startCmdPath)) {
    Write-StartScript
  }

  if ($Wait) {
    Write-Log STEP '以前台控制台方式启动无头服务器...'
    Start-Process -FilePath $startCmdPath -WorkingDirectory $script:GameRoot
  }
  else {
    Write-Log STEP '后台启动无头服务器...'
    Start-Process -FilePath $startCmdPath -ArgumentList 'background' -WorkingDirectory $script:GameRoot
  }
  Write-Log OK '无头服务器启动命令已发出'
}

function Stop-HeadlessServer {
  $proc = Get-ServerProcess
  if (-not $proc) {
    Write-Log WARN '当前没有检测到运行中的 DSPGAME 进程'
    return
  }

  Stop-Process -Id $proc.Id -Force
  Write-Log OK "已停止服务进程，PID: $($proc.Id)"
}

function Show-LogTail {
  param(
    [string]$Path,
    [int]$Lines = 60
  )

  if (-not (Test-Path -LiteralPath $Path)) {
    Write-Log WARN "日志不存在: $Path"
    return
  }

  Write-Host ''
  Write-Host "===== $Path =====" -ForegroundColor DarkCyan
  Get-Content -LiteralPath $Path -Tail $Lines
  Write-Host ''
}

function Get-EnvironmentStatus {
  param([switch]$Online)

  $gameRoot = Test-GameRoot
  $bepCore = Test-Path -LiteralPath (Join-Path $script:BepInExRoot 'core\BepInEx.dll')
  $preloader = Test-Path -LiteralPath (Join-Path $script:BepInExRoot 'core\BepInEx.Preloader.dll')
  $winhttp = Test-Path -LiteralPath (Join-Path $script:GameRoot 'winhttp.dll')
  $doorstop = Test-Path -LiteralPath (Join-Path $script:GameRoot 'doorstop_config.ini')
  $steamAppId = Test-Path -LiteralPath (Join-Path $script:GameRoot 'steam_appid.txt')
  $nebulaDir = Test-Path -LiteralPath (Join-Path $script:PluginsRoot 'nebula-NebulaMultiplayerMod')
  $nebulaFiles = @('NebulaModel.dll', 'NebulaNetwork.dll', 'NebulaPatcher.dll', 'NebulaWorld.dll')
  $missingNebulaFiles = @()
  foreach ($file in $nebulaFiles) {
    if (-not (Test-Path -LiteralPath (Join-Path $script:PluginsRoot "nebula-NebulaMultiplayerMod\$file"))) {
      $missingNebulaFiles += $file
    }
  }
  $startScript = Test-Path -LiteralPath (Join-Path $script:GameRoot 'start-headless.cmd')
  $serverProc = Get-ServerProcess
  $hostPort = Get-NebulaHostPort
  $portState = Test-PortListening -Port $hostPort

  $onlineOk = $null
  if ($Online) {
    $onlineOk = Test-Internet
  }

  $issues = @()
  if (-not $gameRoot.Ok) { $issues += "游戏根目录不完整: $($gameRoot.Missing -join ', ')" }
  if (-not $bepCore) { $issues += '缺少 BepInEx core\BepInEx.dll' }
  if (-not $preloader) { $issues += '缺少 BepInEx.Preloader.dll' }
  if (-not $winhttp) { $issues += '缺少 winhttp.dll，Doorstop 无法注入 BepInEx' }
  if (-not $doorstop) { $issues += '缺少 doorstop_config.ini' }
  if (-not $steamAppId) { $issues += '缺少 steam_appid.txt' }
  if (-not $nebulaDir) { $issues += '缺少 NebulaMultiplayerMod 插件目录' }
  if ($missingNebulaFiles.Count -gt 0) { $issues += "Nebula 插件文件不完整: $($missingNebulaFiles -join ', ')" }
  if (-not $startScript) { $issues += '缺少 start-headless.cmd 启动脚本' }
  if ($Online -and -not $onlineOk) { $issues += '无法连接 Thunderstore，在线安装/修复会失败' }
  if ($serverProc -and -not $portState.Any) { $issues += "DSPGAME 正在运行，但未检测到端口 $hostPort 的 TCP/UDP 监听" }

  [PSCustomObject]@{
    GameRootOk = $gameRoot.Ok
    GameRootMissing = $gameRoot.Missing
    BepInExOk = ($bepCore -and $preloader -and $winhttp -and $doorstop)
    NebulaOk = ($nebulaDir -and $missingNebulaFiles.Count -eq 0)
    SteamAppIdOk = $steamAppId
    StartScriptOk = $startScript
    ServerRunning = [bool]$serverProc
    ServerPid = if ($serverProc) { $serverProc.Id } else { $null }
    HostPort = $hostPort
    PortListening = $portState.Any
    TcpListening = $portState.Tcp
    UdpListening = $portState.Udp
    OnlineOk = $onlineOk
    Issues = $issues
    Ok = ($issues.Count -eq 0)
  }
}

function Show-EnvironmentStatus {
  param([switch]$Online)

  $status = Get-EnvironmentStatus -Online:$Online
  Write-Banner -Title 'DSP Nebula Headless 环境检查'
  Write-Host ''
  Write-Host '===== 环境检查结果 =====' -ForegroundColor Cyan
  Write-Host "游戏目录: $script:GameRoot"
  Write-Host "游戏根目录: $(if ($status.GameRootOk) { '正常' } else { '异常' })"
  Write-Host "BepInEx 注入环境: $(if ($status.BepInExOk) { '正常' } else { '异常' })"
  Write-Host "Nebula 无头服务模组: $(if ($status.NebulaOk) { '正常' } else { '异常' })"
  Write-Host "steam_appid.txt: $(if ($status.SteamAppIdOk) { '存在' } else { '缺失' })"
  Write-Host "start-headless.cmd: $(if ($status.StartScriptOk) { '存在' } else { '缺失' })"
  if ($Online) {
    Write-Host "Thunderstore 网络: $(if ($status.OnlineOk) { '可连接' } else { '不可连接' })"
  }
  Write-Host "服务器状态: $(if ($status.ServerRunning) { "运行中，PID $($status.ServerPid)" } else { '未运行' })"
  Write-Host "Nebula 端口: $($status.HostPort) ($(if ($status.PortListening) { "监听中，TCP=$($status.TcpListening), UDP=$($status.UdpListening)" } else { '未监听' }))"

  if ($status.Issues.Count -eq 0) {
    Write-Log OK '未发现环境问题'
  }
  else {
    Write-Host ''
    Write-Log WARN '发现以下问题:'
    foreach ($issue in $status.Issues) {
      Write-Host "  - $issue" -ForegroundColor Yellow
    }
  }

  return $status
}

function Install-HeadlessEnvironment {
  param([switch]$RepairOnly)

  Assert-GameRoot
  Ensure-Directory -Path $script:WorkRoot
  Ensure-Directory -Path $script:DownloadRoot
  Ensure-Directory -Path $script:PluginsRoot

  $current = Get-EnvironmentStatus
  $needsInstall = (-not $current.BepInExOk) -or (-not $current.NebulaOk) -or $ForceReinstall -or (-not $RepairOnly)

  if (-not $needsInstall) {
    Write-Log OK '环境主体已安装，仅执行配置修复'
  }
  else {
    Backup-ExistingInstall

    $requiredPackages = Resolve-RequiredPackages
    $rootDeps = @()
    foreach ($pkg in $requiredPackages) {
      $rootDeps += $pkg.Dependencies
    }
    $resolvedDependencies = Resolve-PackageGraph -RootDependencies $rootDeps

    $bepInExPackage = $requiredPackages | Where-Object { $_.Owner -eq 'xiaoye97' -and $_.Name -eq 'BepInEx' } | Select-Object -First 1
    $mainModPackage = $requiredPackages | Where-Object { $_.Owner -eq 'nebula' -and $_.Name -eq 'NebulaMultiplayerMod' } | Select-Object -First 1

    Write-Log INFO "BepInEx 版本: $($bepInExPackage.Version)"
    Write-Log INFO "Nebula 版本: $($mainModPackage.Version)"
    if ($resolvedDependencies.Count -gt 0) {
      Write-Log INFO '将安装以下依赖链:'
      foreach ($dep in $resolvedDependencies) {
        Write-Log INFO "  $($dep.RequestedDependency) => $($dep.Owner)/$($dep.Name) $($dep.Version)"
      }
    }

    Install-BepInExPack -Package $bepInExPackage
    foreach ($pkg in $resolvedDependencies) {
      if ($pkg.Owner -eq 'xiaoye97' -and $pkg.Name -eq 'BepInEx') {
        continue
      }
      Install-ModPackage -Package $pkg
    }

    if (-not ($resolvedDependencies | Where-Object { $_.Owner -eq $mainModPackage.Owner -and $_.Name -eq $mainModPackage.Name })) {
      Install-ModPackage -Package $mainModPackage
    }
  }

  Ensure-SteamAppId
  Ensure-DoorstopConfig
  Initialize-BepInExConfig
  Ensure-BepInExConsole
  Write-StartScript

  Write-Log OK '环境依赖安装/修复完成'
  if ($StartAfterInstall) {
    Start-HeadlessServer
  }
}

function Show-LogsMenu {
  $bepLog = Join-Path $script:BepInExRoot 'LogOutput.log'
  $unityLog = Join-Path $script:GameRoot 'output_log.txt'
  Show-LogTail -Path $bepLog
  Show-LogTail -Path $unityLog
}

function Show-ManagementConsole {
  $saveDir = Join-Path $env:USERPROFILE 'Documents\Dyson Sphere Program\Save'
  $bepLog = Join-Path $script:BepInExRoot 'LogOutput.log'
  $unityLog = Join-Path $script:GameRoot 'output_log.txt'
  $startCmdPath = Join-Path $script:GameRoot 'start-headless.cmd'

  while ($true) {
    Clear-Host
    $status = Get-EnvironmentStatus

    Write-Banner -Title 'DSP Nebula Headless 环境控制台'
    Write-Host "游戏目录: $script:GameRoot" -ForegroundColor White
    Write-Host "默认存档: $SaveName" -ForegroundColor White
    Write-Host "存档目录: $saveDir" -ForegroundColor White
    Write-Host "启动脚本: $startCmdPath" -ForegroundColor White
    Write-Host "BepInEx 日志: $bepLog" -ForegroundColor White
    Write-Host "Unity 日志: $unityLog" -ForegroundColor White
    Write-Host ''
    Write-Host "环境状态: $(if ($status.Ok) { '正常' } else { '需要处理' })" -ForegroundColor $(if ($status.Ok) { 'Green' } else { 'Yellow' })
    Write-Host "服务状态: $(if ($status.ServerRunning) { "运行中，PID $($status.ServerPid)" } else { '未运行' })" -ForegroundColor $(if ($status.ServerRunning) { 'Green' } else { 'Yellow' })
    Write-Host "端口监听: $($status.HostPort) $(if ($status.PortListening) { "(监听中 TCP=$($status.TcpListening) UDP=$($status.UdpListening))" } else { '(未监听)' })" -ForegroundColor $(if ($status.PortListening) { 'Green' } elseif ($status.ServerRunning) { 'Red' } else { 'Yellow' })
    Write-Host ''
    Write-Host '[1] 本地检查环境' -ForegroundColor Cyan
    Write-Host '[2] 在线检查环境（含 Thunderstore 连通性）' -ForegroundColor Cyan
    Write-Host '[3] 安装/更新无头服务器环境' -ForegroundColor Cyan
    Write-Host '[4] 检查修复环境（缺什么补什么）' -ForegroundColor Cyan
    Write-Host '[5] 强制重装环境（先备份当前模组环境）' -ForegroundColor Cyan
    Write-Host '[6] 重新生成启动脚本' -ForegroundColor Cyan
    Write-Host '[7] 启动服务（后台）' -ForegroundColor Cyan
    Write-Host '[8] 启动服务（前台控制台）' -ForegroundColor Cyan
    Write-Host '[9] 停止服务' -ForegroundColor Cyan
    Write-Host '[10] 查看日志尾部' -ForegroundColor Cyan
    Write-Host '[11] 打开游戏目录' -ForegroundColor Cyan
    Write-Host '[12] 打开存档目录' -ForegroundColor Cyan
    Write-Host '[0] 退出控制台' -ForegroundColor Cyan
    Write-Host ''
    Write-Host '请选择操作并回车: ' -NoNewline -ForegroundColor Magenta
    $choice = Read-Host

    try {
      switch ($choice) {
        '1' {
          Show-EnvironmentStatus | Out-Null
          Wait-ForAnyKey
        }
        '2' {
          Show-EnvironmentStatus -Online | Out-Null
          Wait-ForAnyKey
        }
        '3' {
          Install-HeadlessEnvironment
          Wait-ForAnyKey
        }
        '4' {
          Install-HeadlessEnvironment -RepairOnly
          Wait-ForAnyKey
        }
        '5' {
          $oldForce = $ForceReinstall
          try {
            Set-Variable -Name ForceReinstall -Value $true -Scope Script
            Install-HeadlessEnvironment
          }
          finally {
            Set-Variable -Name ForceReinstall -Value $oldForce -Scope Script
          }
          Wait-ForAnyKey
        }
        '6' {
          Assert-GameRoot
          Write-StartScript
          Wait-ForAnyKey
        }
        '7' {
          Start-HeadlessServer
          Start-Sleep -Seconds 2
        }
        '8' {
          Start-HeadlessServer -Wait
          Start-Sleep -Seconds 2
        }
        '9' {
          Stop-HeadlessServer
          Wait-ForAnyKey
        }
        '10' {
          Show-LogsMenu
          Wait-ForAnyKey
        }
        '11' {
          Start-Process explorer.exe $script:GameRoot
        }
        '12' {
          Ensure-Directory -Path $saveDir
          Start-Process explorer.exe $saveDir
        }
        '0' {
          break
        }
        default {
          Write-Log WARN "未知选项: $choice"
          Start-Sleep -Seconds 1
        }
      }
    }
    catch {
      Write-Log ERROR $_.Exception.Message
      Wait-ForAnyKey
    }
  }
}

$script:GameRoot = (Get-Location).Path
$script:WorkRoot = Join-Path $script:GameRoot '.headless-install'
$script:DownloadRoot = Join-Path $script:WorkRoot 'downloads'
$script:BepInExRoot = Join-Path $script:GameRoot 'BepInEx'
$script:BepInExConfigRoot = Join-Path $script:BepInExRoot 'config'
$script:PluginsRoot = Join-Path $script:BepInExRoot 'plugins'
$script:PackageIndex = $null

Write-Log INFO "当前目录: $script:GameRoot"

try {
  switch ($Mode) {
    'Console' {
      Show-ManagementConsole
    }
    'Check' {
      Show-EnvironmentStatus -Online | Out-Null
    }
    'Install' {
      Install-HeadlessEnvironment
      if (-not $NoDashboard) {
        Show-ManagementConsole
      }
    }
    'Repair' {
      Install-HeadlessEnvironment -RepairOnly
      if (-not $NoDashboard) {
        Show-ManagementConsole
      }
    }
    'Start' {
      Start-HeadlessServer
    }
    'StartWait' {
      Start-HeadlessServer -Wait
    }
    'Stop' {
      Stop-HeadlessServer
    }
    'Logs' {
      Show-LogsMenu
    }
    'GenerateStartScript' {
      Assert-GameRoot
      Write-StartScript
    }
  }
}
catch {
  Write-Log ERROR $_.Exception.Message
  exit 1
}

