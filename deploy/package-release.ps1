param(
    [Parameter(Mandatory = $true)][ValidatePattern('^v[0-9]+\.[0-9]+\.[0-9]+$')][string]$Version,
    [Parameter(Mandatory = $true)][ValidateSet('quality', 'balanced', 'compact')][string]$Mode
)

# 职责：校验双架构 EXE、生成更新归档、哈希与压缩报告；使用：由 build.bat 在编译成功后调用。
# 边界：只写入版本与模式对应的发布目录，不清空 build/bin，不处理运行数据库、资源库或其他模式产物。
$ErrorActionPreference = 'Stop'
$releaseRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$releaseOutput = [IO.Path]::GetFullPath((Join-Path $releaseRoot "deploy\release\$Version\$Mode"))
$releaseExpectedPrefix = [IO.Path]::GetFullPath((Join-Path $releaseRoot 'deploy\release')) + [IO.Path]::DirectorySeparatorChar
if (-not $releaseOutput.StartsWith($releaseExpectedPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw '发布目录越界' }
[IO.Directory]::CreateDirectory($releaseOutput) | Out-Null
$releaseReportPath = Join-Path $releaseRoot 'frontend\dist\model-compression-report.json'
$releaseReport = [IO.File]::ReadAllText($releaseReportPath) | ConvertFrom-Json
if ($releaseReport.compressionMode -ne $Mode -or $releaseReport.modelCount -lt 1) { throw '前端模型压缩档位与发布模式不一致' }
Add-Type -AssemblyName System.IO.Compression
$releaseArtifacts = @()

# 使用 .NET 流式哈希，兼容未自动加载 Get-FileHash 的 Windows PowerShell 环境。
function Get-ReleaseSha256([string]$Path) {
    $releaseHashAlgorithm = [Security.Cryptography.SHA256]::Create()
    $releaseHashStream = [IO.File]::OpenRead($Path)
    try { return -join ($releaseHashAlgorithm.ComputeHash($releaseHashStream) | ForEach-Object { $_.ToString('x2') }) }
    finally { $releaseHashStream.Dispose(); $releaseHashAlgorithm.Dispose() }
}

foreach ($releaseArch in @('amd64', 'arm64')) {
    $releaseSource = Join-Path $releaseRoot "build\bin\DysonSphereBox-release-$releaseArch.exe"
    # 校验 PE 头，避免复制上一轮或另一架构的文件。
    $releaseBinaryStream = [IO.File]::OpenRead($releaseSource)
    $releaseReader = New-Object IO.BinaryReader($releaseBinaryStream)
    try {
        if ($releaseReader.ReadUInt16() -ne 0x5A4D) { throw '不是有效的 Windows EXE' }
        $releaseBinaryStream.Position = 0x3C
        $releasePeOffset = $releaseReader.ReadInt32()
        if ($releasePeOffset -lt 0 -or $releasePeOffset + 6 -gt $releaseBinaryStream.Length) { throw 'PE 头越界' }
        $releaseBinaryStream.Position = $releasePeOffset
        if ($releaseReader.ReadUInt32() -ne 0x00004550) { throw 'PE 签名错误' }
        $releaseExpectedMachine = if ($releaseArch -eq 'amd64') { 0x8664 } else { 0xAA64 }
        if ($releaseReader.ReadUInt16() -ne $releaseExpectedMachine) { throw "EXE 架构与 $releaseArch 不符" }
    } finally { $releaseReader.Dispose(); $releaseBinaryStream.Dispose() }

    $releaseExe = Join-Path $releaseOutput "DysonSphereBox_windows_$releaseArch.exe"
    Copy-Item -LiteralPath $releaseSource -Destination $releaseExe -Force
    $releaseZip = Join-Path $releaseOutput "DysonSphereBox_windows_$releaseArch.zip"
    # 直接写归档并固定内部名称，不创建临时 EXE 副本或递归删除临时目录。
    $releaseZipStream = [IO.File]::Open($releaseZip, [IO.FileMode]::Create, [IO.FileAccess]::ReadWrite)
    $releaseArchive = New-Object IO.Compression.ZipArchive($releaseZipStream, [IO.Compression.ZipArchiveMode]::Create, $false)
    try {
        $releaseEntry = $releaseArchive.CreateEntry('DysonSphereBox.exe', [IO.Compression.CompressionLevel]::Optimal)
        $releaseEntryStream = $releaseEntry.Open()
        $releaseInputStream = [IO.File]::OpenRead($releaseExe)
        try { $releaseInputStream.CopyTo($releaseEntryStream) }
        finally { $releaseInputStream.Dispose(); $releaseEntryStream.Dispose() }
    } finally { $releaseArchive.Dispose(); $releaseZipStream.Dispose() }
    foreach ($releaseFile in @($releaseExe, $releaseZip)) {
        $releaseInfo = Get-Item -LiteralPath $releaseFile
        $releaseArtifacts += [PSCustomObject]@{name=$releaseInfo.Name; bytes=$releaseInfo.Length; sha256=(Get-ReleaseSha256 $releaseFile)}
    }
}

$releaseChecksums = @($releaseArtifacts | ForEach-Object { $_.sha256 + '  ' + $_.name })
[IO.File]::WriteAllLines((Join-Path $releaseOutput 'checksums.txt'), [string[]]$releaseChecksums, [Text.Encoding]::ASCII)
Copy-Item -LiteralPath $releaseReportPath -Destination (Join-Path $releaseOutput 'model-compression-report.json') -Force
$releaseProfile = [ordered]@{
    version=$Version; compressionMode=$Mode; maxTextureSize=$releaseReport.maxTextureSize
    modelCount=$releaseReport.modelCount; originalModelBytes=$releaseReport.beforeBytes; compressedModelBytes=$releaseReport.afterBytes
    createdAt=[DateTime]::UtcNow.ToString('o'); artifacts=$releaseArtifacts
}
[IO.File]::WriteAllText((Join-Path $releaseOutput 'build-profile.json'), ($releaseProfile | ConvertTo-Json -Depth 4), (New-Object Text.UTF8Encoding($false)))
$releaseArtifacts | Format-Table name, bytes, sha256 -AutoSize
Write-Output "Release files created in: $releaseOutput"
