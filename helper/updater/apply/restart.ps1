# 职责：等待旧进程退出后安装新版、删除旧包体并自动启动程序。
# 使用：由 StartAfterExit 传入本地生成的 plan；边界：只处理明确的暂存、备份和目标路径。
$ErrorActionPreference = 'Stop'
$target = [string]$plan.target
$directory = [string]$plan.directory
$staged = [IO.Path]::Combine($directory, 'update.exe')
$backup = [IO.Path]::Combine($directory, 'previous.exe')
$ready = [IO.Path]::Combine($directory, 'ready')
$parent = [Diagnostics.Process]::GetProcessById([int]$plan.parentPID)
$null = $parent.Handle
[IO.File]::WriteAllText($ready, 'ready')
$parent.WaitForExit()
$parent.Dispose()

try {
    [IO.File]::Move($target, $backup)
    try {
        [IO.File]::Move($staged, $target)
    } catch {
        [IO.File]::Move($backup, $target)
        throw
    }

    # 新版已到位，删除本轮旧包体和旧更新器遗留的同名备份。
    [IO.File]::Delete($backup)
    $legacyBackup = [IO.Path]::Combine([IO.Path]::GetDirectoryName($target), '.' + [IO.Path]::GetFileName($target) + '.old')
    [IO.File]::Delete($legacyBackup)
    [IO.File]::Delete($ready)
    [IO.Directory]::Delete($directory, $false)
    Start-Process -FilePath $target -WorkingDirectory ([IO.Path]::GetDirectoryName($target))
} catch {
    $reason = $_.Exception.Message
    $errorPath = $target + '.update-error.txt'
    [IO.File]::WriteAllText($errorPath, $reason, [Text.Encoding]::UTF8)
    # 替换失败已恢复原文件；错误明确展示，避免静默退出后让用户猜测。
    Add-Type -AssemblyName System.Windows.Forms
    [Windows.Forms.MessageBox]::Show("更新未完成：" + $reason + [Environment]::NewLine + "错误详情：" + $errorPath, '戴森球盒子更新', 'OK', 'Error') | Out-Null
    if ([IO.File]::Exists($target)) {
        Start-Process -FilePath $target -WorkingDirectory ([IO.Path]::GetDirectoryName($target))
    }
    exit 1
}
