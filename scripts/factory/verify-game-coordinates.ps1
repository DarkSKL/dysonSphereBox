<#
.SYNOPSIS
  直接调用本机游戏 Assembly-CSharp.dll 核对球面蓝图坐标契约。

.DESCRIPTION
  职责：
    1. 加载游戏 Managed/UnityEngine.CoreModule.dll 与 Assembly-CSharp.dll，读取经度段数原生结果；
    2. 生成或核对 helper/blueprint/build/testdata/game-grid-25955035.json 数值 fixture；
    3. 用 GetLatitudeRadPerGrid/GetLongitudeRadPerGrid/GetDir 与 Vector3*float 生成 worldPointSamples 世界坐标；
    4. 用最小多 Area 样本证明 GenerateAreaGratBoxByBPData 的四类 Area 条件确实接受/拒绝；
    5. 可选核对本仓库 blueprint.Document JSON（或 {document, surfacePlan} 包装）在指定根纬向格的 Area 条件。
  使用：
    pwsh -NoProfile -File scripts/factory/verify-game-coordinates.ps1 [-GameDirectory <dir>]
         [-BlueprintDocumentPath <file|dir>] [-RootLatitudeGrid <int>] [-UpdateFixture]
  边界：
    - 只读游戏文件；不写游戏目录或存档，DLL 仅在当前 pwsh 进程中加载。
    - 只检验 yaw=0、主 Area 起点经度格 0 时的 Area 条件；不调用依赖 Unity 场景的 SnapTropic、碰撞或 Physics。
    - worldPointSamples 是 DLL 静态函数直接求值，不是 Unity 场景内粘贴预览实测；不含 yaw 旋转、地形高度和科技修正。
    - 数值在 pwsh 的 .NET 运行时求值，不是 Unity Mono；float 运算均为 IEEE 单精度，fixture 记录实际运行时。
    - 游戏 float/Vector3 版经度段 API 吸附纬向格时固定使用默认 200 分段，因此只宣称支持 segment=200。
  退出码：0 全部通过；1 任一检查失败或运行错误。
#>
#Requires -Version 7
[CmdletBinding()]
param(
    [string]$GameDirectory = 'E:/Games/Steam/steamapps/common/Dyson Sphere Program',
    [string]$BlueprintDocumentPath,
    [Nullable[int]]$RootLatitudeGrid = $null,
    [string]$FixturePath = (Join-Path $PSScriptRoot '../../helper/blueprint/build/testdata/game-grid-25955035.json'),
    [switch]$UpdateFixture
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
# 输出编码 职责：保证中文日志在重定向或被其他进程捕获时为 UTF-8；边界：只影响当前 pwsh 进程。
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)

# 脚本常量 职责：固定 fixture 对应的游戏构建与全范围分段；边界：其他构建需重新生成 fixture 并复核。
$ExpectedSteamBuildId = '25955035'
$FullTableSegment = 200
# 游戏 BuildTool_BlueprintPaste 为 Area 条件分配 IntVector4[64]，超过即不可粘贴。
$MaxPasteAreas = 64

$script:Failures = 0

# Write-Check 职责：统一输出检查结果并累计失败；使用：所有样本与文档检查；边界：只计数，不中断后续检查。
function Write-Check([bool]$ok, [string]$message) {
    if ($ok) {
        Write-Host "[PASS] $message"
    } else {
        $script:Failures++
        Write-Host "[FAIL] $message"
    }
}

# Get-OptionalProperty 职责：在 StrictMode 下安全读取 JSON 可选字段；使用：文档与 surfacePlan 解析；边界：缺失返回 $null。
function Get-OptionalProperty($object, [string]$name) {
    if ($null -ne $object -and $object.PSObject.Properties[$name]) {
        return $object.$name
    }
    return $null
}

# Get-FileSha256 职责：生成 DLL 来源指纹；使用：fixture source 与核对；边界：只读文件。
function Get-FileSha256([string]$path) {
    return (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
}

# Get-GameMethod 职责：按精确参数类型取游戏方法，避免 PowerShell 重载推断取错 int/float 版本；
# 使用：所有原生调用；边界：找不到即视为游戏 API 变化并报错。
function Get-GameMethod([type]$type, [string]$name, [type[]]$parameterTypes) {
    $method = $type.GetMethod($name, [Reflection.BindingFlags]'Public, Static', $null, $parameterTypes, $null)
    if ($null -eq $method) {
        throw "游戏 API 不存在：$($type.FullName).$name($($parameterTypes.Name -join ', '))"
    }
    return $method
}

# Format-FixtureJson 职责：生成稳定、紧凑的 fixture 文本；使用：写入与比较；边界：只把纯整数数组折叠到一行。
function Format-FixtureJson($value) {
    $json = $value | ConvertTo-Json -Depth 12
    $json = [regex]::Replace($json, '\[\s*(-?\d+(?:\s*,\s*-?\d+)*)\s*\]', {
            param($match)
            '[' + (($match.Groups[1].Value -split '\s*,\s*') -join ',') + ']'
        })
    return ($json -replace "`r`n", "`n") + "`n"
}

# ---------- 1. 加载游戏程序集（只读） ----------
$managed = Join-Path $GameDirectory 'DSPGAME_Data/Managed'
$assemblyPath = Join-Path $managed 'Assembly-CSharp.dll'
$corePath = Join-Path $managed 'UnityEngine.CoreModule.dll'
try {
    foreach ($path in @($corePath, $assemblyPath)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "缺少游戏程序集：$path"
        }
    }
    $null = [Reflection.Assembly]::LoadFrom((Resolve-Path -LiteralPath $corePath).Path)
    $gameAssembly = [Reflection.Assembly]::LoadFrom((Resolve-Path -LiteralPath $assemblyPath).Path)
} catch {
    Write-Host "[ERROR] 加载游戏程序集失败：$($_.Exception.Message)"
    exit 1
}

$tPlanetGrid = $gameAssembly.GetType('PlanetGrid', $true)
$tUtils = $gameAssembly.GetType('BlueprintUtils', $true)
$tBlueprintData = $gameAssembly.GetType('BlueprintData', $true)
$tBlueprintArea = $gameAssembly.GetType('BlueprintArea', $true)
$tGratBox = $gameAssembly.GetType('BPGratBox', $true)
$tIntVector4 = $gameAssembly.GetType('IntVector4', $true)
$tVector3 = [UnityEngine.Vector3]

$mDetermine = Get-GameMethod $tPlanetGrid 'DetermineLongitudeSegmentCount' @([int], [int])
$mCountInt = Get-GameMethod $tUtils 'GetLongitudeSegmentCount' @([int], [int])
$mCountFloat = Get-GameMethod $tUtils 'GetLongitudeSegmentCount' @([single], [int])
$mLatRadPerGrid = Get-GameMethod $tUtils 'GetLatitudeRadPerGrid' @([int])
$mLonRadPerGrid = Get-GameMethod $tUtils 'GetLongitudeRadPerGrid' @([int], [int])
$mGetDir = Get-GameMethod $tUtils 'GetDir' @([single], [single])
$mVectorScale = Get-GameMethod $tVector3 'op_Multiply' @($tVector3, [single])
$mGenerate = Get-GameMethod $tUtils 'GenerateAreaGratBoxByBPData' @(
    $tBlueprintData, $tGratBox.MakeArrayType(), $tIntVector4.MakeArrayType(), $tVector3.MakeArrayType(), [int], [single], [int])
$fSegmentTable = $tPlanetGrid.GetField('segmentTable', [Reflection.BindingFlags]'Public, Static')
if ($null -eq $fSegmentTable) {
    Write-Host '[ERROR] 游戏 PlanetGrid.segmentTable 不存在'
    exit 1
}

$assemblySha = Get-FileSha256 $assemblyPath
$coreSha = Get-FileSha256 $corePath
$gameConfig = $gameAssembly.GetType('GameConfig', $false)
$gameVersion = if ($gameConfig) { [string]$gameConfig.GetProperty('gameVersion').GetValue($null) } else { $null }
$steamBuildId = $null
$manifest = Join-Path $GameDirectory '../../appmanifest_1366540.acf'
if (Test-Path -LiteralPath $manifest -PathType Leaf) {
    $buildLine = Select-String -LiteralPath $manifest -Pattern '"buildid"\s+"(\d+)"' | Select-Object -First 1
    if ($buildLine) { $steamBuildId = $buildLine.Matches[0].Groups[1].Value }
}
$runtime = [Runtime.InteropServices.RuntimeInformation]::FrameworkDescription

Write-Host "游戏程序集：$assemblyPath"
Write-Host "  sha256=$assemblySha gameVersion=$gameVersion steamBuildId=$steamBuildId runtime=$runtime"
Write-Host "  UnityEngine.CoreModule sha256=$coreSha"
foreach ($method in @($mDetermine, $mCountInt, $mCountFloat, $mLatRadPerGrid, $mLonRadPerGrid, $mGetDir, $mVectorScale, $mGenerate)) {
    Write-Host "  方法：$($method.DeclaringType.Name).$method"
}
if ($steamBuildId -and $steamBuildId -ne $ExpectedSteamBuildId) {
    Write-Host "[WARN] 本机 Steam buildid $steamBuildId 与 fixture 构建 $ExpectedSteamBuildId 不同，核对结果以 DLL sha256 为准"
}

# 原生调用包装 职责：固定参数类型调用游戏函数；使用：fixture 与样本；边界：不缓存结果，保证每次都来自 DLL。
function Get-CountInt([int]$grid, [int]$segment) { return [int]$mCountInt.Invoke($null, @($grid, $segment)) }
function Get-CountFloat([single]$rad, [int]$segment) { return [int]$mCountFloat.Invoke($null, @($rad, $segment)) }
function Get-LatRadPerGrid([int]$segment) { return [single]$mLatRadPerGrid.Invoke($null, @($segment)) }

# ---------- 2. 生成 fixture 数值 ----------
# Get-GridLatitudeRad 职责：按游戏单精度乘法求纬向格弧度；边界：float*float 先得到精确 double 积再舍入，等同 IEEE 单精度乘法。
function Get-GridLatitudeRad([int]$grid, [int]$segment) {
    return [single]([single]$grid * (Get-LatRadPerGrid $segment))
}

$segmentTable = @($fSegmentTable.GetValue($null))
$maxGrid = [int]($FullTableSegment * 5 / 4)
$intApi = [Collections.Generic.List[int]]::new()
$floatApi = [Collections.Generic.List[int]]::new()
$latitudeIndexApi = [Collections.Generic.List[int]]::new()
$mappingMismatches = 0
for ($grid = -$maxGrid; $grid -le $maxGrid; $grid++) {
    $countInt = Get-CountInt $grid $FullTableSegment
    $intApi.Add($countInt)
    $floatApi.Add((Get-CountFloat (Get-GridLatitudeRad $grid $FullTableSegment) $FullTableSegment))
    # 游戏 int 版映射：|grid|，非零减一，再整除 5 得 latitudeIndex。
    $folded = [Math]::Abs($grid)
    if ($folded -gt 0) { $folded-- }
    if ($countInt -ne [int]$mDetermine.Invoke($null, @([int][Math]::Floor($folded / 5), $FullTableSegment))) { $mappingMismatches++ }
}
for ($index = 0; $index -le $FullTableSegment / 4; $index++) {
    $latitudeIndexApi.Add([int]$mDetermine.Invoke($null, @($index, $FullTableSegment)))
}
$floatMismatch200 = 0
for ($i = 0; $i -lt $intApi.Count; $i++) { if ($intApi[$i] -ne $floatApi[$i]) { $floatMismatch200++ } }

# 纬带 职责：把 int API 结果压缩为连续等段数纬带；使用：Go 选择纬带和样本构造；边界：仅 segment=200。
$bands = [Collections.Generic.List[object]]::new()
$bandStart = -$maxGrid
for ($i = 1; $i -le $intApi.Count; $i++) {
    if ($i -eq $intApi.Count -or $intApi[$i] -ne $intApi[$i - 1]) {
        $bands.Add([ordered]@{ minLatitudeGrid = $bandStart; maxLatitudeGrid = $i - 1 - $maxGrid; longitudeSegments = $intApi[$i - 1] })
        $bandStart = $i - $maxGrid
    }
}

# float API 默认 200 审计 职责：记录其他主网格分段下 float 版与 int 版的差异；边界：零差异也不等于支持该分段。
$floatAudit = [Collections.Generic.List[object]]::new()
for ($segment = 40; $segment -le 512; $segment += 4) {
    $limit = [int][Math]::Floor($segment * 5 / 4)
    $mismatch = 0
    $first = $null
    for ($grid = -$limit; $grid -le $limit; $grid++) {
        $a = Get-CountInt $grid $segment
        $b = Get-CountFloat (Get-GridLatitudeRad $grid $segment) $segment
        if ($a -ne $b) {
            $mismatch++
            if ($null -eq $first) { $first = [ordered]@{ latitudeGrid = $grid; intApi = $a; floatApi = $b } }
        }
    }
    $floatAudit.Add([ordered]@{ segment = $segment; latitudeGridMax = $limit; mismatchCount = $mismatch; firstMismatch = $first })
}

# 世界坐标样本 职责：按游戏 float32 函数把单区域局部坐标映射到球面世界坐标，供 Go surfaceWorldPositions 独立对照；
# 使用：fixture worldPointSamples；边界：主区域起点经度 0、yaw=0、localY=0、原版半径 200，DLL 静态函数求值而非 Unity 场景。
# 单精度字面量按 C# 'f' 后缀语义直接从十进制解析，避免先转 double 再转 float 的二次舍入。
$heightStep = [single]::Parse('1.3333333', [Globalization.CultureInfo]::InvariantCulture)
$heightBias = [single]::Parse('0.2', [Globalization.CultureInfo]::InvariantCulture)
$planetRadius = [single]200
# Format-FloatBits 职责：输出 float32 位模式；边界：只格式化。
function Format-FloatBits([single]$value) { return '0x{0:x8}' -f [BitConverter]::SingleToInt32Bits($value) }
$worldPoints = [Collections.Generic.List[object]]::new()
$latRadPerGrid = Get-LatRadPerGrid $FullTableSegment
foreach ($anchor in 0, 80, 100, 200, -100) {
    $segments = Get-CountInt $anchor $FullTableSegment
    $lonRadPerGrid = [single]$mLonRadPerGrid.Invoke($null, @($segments, $FullTableSegment))
    foreach ($localX in 0, 1, 10) {
        foreach ($localZ in 0, 1, 3) {
            $localY = 0
            # float*float 与这些量级的 float+float 在 double 中精确，再舍入一次即 IEEE 单精度结果。
            $latitude = [single]([single]$anchor * $latRadPerGrid + [single]([single]$localY * $latRadPerGrid))
            $longitude = [single]([single]$localX * $lonRadPerGrid)
            $radius = [single]([single]([single]([single]$localZ * $heightStep) + $heightBias) + $planetRadius)
            $dir = $mGetDir.Invoke($null, @($longitude, $latitude))
            $world = $mVectorScale.Invoke($null, @($dir, $radius))
            $worldPoints.Add([ordered]@{
                    anchorLatitudeGrid = $anchor
                    longitudeSegments  = $segments
                    localOffset        = [ordered]@{ x = $localX; y = $localY; z = $localZ }
                    latitudeRadFloatBits = Format-FloatBits $latitude
                    longitudeRadFloatBits = Format-FloatBits $longitude
                    radiusFloatBits    = Format-FloatBits $radius
                    worldPosition      = [ordered]@{ x = [single]$world.x; y = [single]$world.y; z = [single]$world.z }
                    worldPositionFloatBits = [ordered]@{ x = Format-FloatBits $world.x; y = Format-FloatBits $world.y; z = Format-FloatBits $world.z }
                })
        }
    }
}

$latRad200 = Get-LatRadPerGrid $FullTableSegment
$fixture = [ordered]@{
    schema      = 'dsbox.gameGridFixture/v1'
    description = '由 scripts/factory/verify-game-coordinates.ps1 直接调用游戏 DLL 生成的经度分段数值事实，不含反编译源码。'
    source      = [ordered]@{
        assembly             = 'DSPGAME_Data/Managed/Assembly-CSharp.dll'
        assemblySha256       = $assemblySha
        unityCoreModuleSha256 = $coreSha
        gameConfigVersion    = $gameVersion
        steamBuildId         = $steamBuildId
        evaluationRuntime    = $runtime
    }
    methods     = [ordered]@{
        latitudeIndex     = "PlanetGrid.$mDetermine"
        latitudeGrid      = "BlueprintUtils.$mCountInt"
        latitudeRad       = "BlueprintUtils.$mCountFloat"
        latitudeRadPerGrid = "BlueprintUtils.$mLatRadPerGrid"
        areaConditions    = "BlueprintUtils.$mGenerate"
        segmentTable      = 'PlanetGrid.segmentTable (public static int[], reflection)'
        latitudeGridMapping = 'latitudeIndex = (|latitudeGrid| - (latitudeGrid != 0 ? 1 : 0)) / 5'
        floatApiNote      = 'GetLongitudeSegmentCount(float|Vector3, segmentCnt) 吸附纬向格时调用 GetSnappedLatitudeGridIdx(rad) 使用默认 200；GenerateAreaGratBoxByBPData 内部经度格宽与段数比例检查走该路径。'
    }
    segmentTable = [ordered]@{
        source = 'PlanetGrid.segmentTable'
        length = $segmentTable.Count
        values = $segmentTable
    }
    segment200   = [ordered]@{
        segment                   = $FullTableSegment
        latitudeRadPerGridFloatBits = ('0x{0:x8}' -f [BitConverter]::SingleToInt32Bits($latRad200))
        latitudeGridMin           = -$maxGrid
        latitudeGridMax           = $maxGrid
        longitudeSegmentsByLatitudeGrid = $intApi.ToArray()
        longitudeSegmentsByLatitudeRad  = $floatApi.ToArray()
        determineLongitudeSegmentCountByLatitudeIndex = $latitudeIndexApi.ToArray()
        bands                     = $bands.ToArray()
    }
    floatApiSegmentAudit = [ordered]@{
        note             = '比较 GetLongitudeSegmentCount(int grid, seg) 与 GetLongitudeSegmentCount((float)grid*GetLatitudeRadPerGrid(seg), seg)；不一致的分段不能用游戏 Area 检查结论，仍只宣称支持 segment=200。'
        supportedSegments = @($FullTableSegment)
        results          = $floatAudit.ToArray()
    }
    worldPointSamples = [ordered]@{
        note    = '单区域（主区域起点经度格 0、yaw=0、segment=200）局部坐标到世界坐标：latitude = (float)(anchorLatitudeGrid) * GetLatitudeRadPerGrid(200) + localY * 同值；longitude = localX * GetLongitudeRadPerGrid(longitudeSegments, 200)；radius = localZ * 1.3333333f + 0.2f + 200f；worldPosition = GetDir(longitude, latitude) * radius（UnityEngine.Vector3.op_Multiply）。十进制值为 float32 最短可往返表示，*FloatBits 为同值位模式。DLL 静态函数直接求值，非 Unity 场景粘贴实测。'
        methods = [ordered]@{
            latitudeRadPerGrid  = "BlueprintUtils.$mLatRadPerGrid"
            longitudeRadPerGrid = "BlueprintUtils.$mLonRadPerGrid"
            dir                 = "BlueprintUtils.$mGetDir"
            scale               = "Vector3.$mVectorScale"
            longitudeSegments   = "BlueprintUtils.$mCountInt"
        }
        samples = $worldPoints.ToArray()
    }
}

Write-Check ($segmentTable.Count -eq 512) "PlanetGrid.segmentTable 长度=$($segmentTable.Count)"
Write-Check ($mappingMismatches -eq 0) "segment200 int API 与 DetermineLongitudeSegmentCount 映射一致（-$maxGrid..$maxGrid，共 $($intApi.Count) 行，差异 $mappingMismatches）"
Write-Check ($floatMismatch200 -eq 0) "segment200 float API 与 int API 一致（差异 $floatMismatch200）"
$auditBad = @($floatAudit | Where-Object { $_.mismatchCount -gt 0 })
Write-Host "  float API 默认 200 审计：40..512 步长 4 共 $($floatAudit.Count) 个分段，$($auditBad.Count) 个存在差异：$((@($auditBad | ForEach-Object { "$($_.segment)($($_.mismatchCount))" })) -join ' ')"

Write-Host "  worldPointSamples：$($worldPoints.Count) 个（纬向格 0/80/100/200/-100 × localX 0/1/10 × localZ 0/1/3，localY=0）"

# Read-WorldPointKeys 职责：从 fixture 文本按 float32 直接解析 worldPointSamples，得到每样本的可比较键；
# 使用：往返检查与 fixture 专项比较；边界：用 System.Text.Json GetSingle 避免 double 二次舍入，缺字段即抛错。
function Read-WorldPointKeys([string]$text) {
    $doc = [Text.Json.JsonDocument]::Parse($text)
    try {
        $keys = foreach ($sample in $doc.RootElement.GetProperty('worldPointSamples').GetProperty('samples').EnumerateArray()) {
            $offset = $sample.GetProperty('localOffset')
            $position = $sample.GetProperty('worldPosition')
            $bits = $sample.GetProperty('worldPositionFloatBits')
            $decimal = @('x', 'y', 'z' | ForEach-Object { Format-FloatBits $position.GetProperty($_).GetSingle() }) -join ','
            $recorded = @('x', 'y', 'z' | ForEach-Object { $bits.GetProperty($_).GetString() }) -join ','
            [pscustomobject]@{
                key      = "grid=$($sample.GetProperty('anchorLatitudeGrid').GetInt32()) seg=$($sample.GetProperty('longitudeSegments').GetInt32()) local=($($offset.GetProperty('x').GetInt32()),$($offset.GetProperty('y').GetInt32()),$($offset.GetProperty('z').GetInt32()))"
                decimal  = $decimal
                recorded = $recorded
            }
        }
        return @($keys)
    } finally {
        $doc.Dispose()
    }
}

$fixtureText = Format-FixtureJson $fixture
$generatedPoints = Read-WorldPointKeys $fixtureText
$roundTripBad = @($generatedPoints | Where-Object { $_.decimal -ne $_.recorded })
Write-Check ($generatedPoints.Count -eq $worldPoints.Count -and $roundTripBad.Count -eq 0) "worldPosition 十进制文本按 float32 解析后与位模式一致（$($generatedPoints.Count) 个样本，差异 $($roundTripBad.Count)）"
$fixtureFull = [IO.Path]::GetFullPath($FixturePath)
if ($UpdateFixture) {
    $null = New-Item -ItemType Directory -Force -Path (Split-Path -Parent $fixtureFull)
    [IO.File]::WriteAllText($fixtureFull, $fixtureText, [Text.UTF8Encoding]::new($false))
    Write-Host "  已写入 fixture：$fixtureFull"
} elseif (Test-Path -LiteralPath $fixtureFull -PathType Leaf) {
    # 比较时忽略求值运行时描述，只要求 DLL 指纹和全部数值一致。
    $strip = { param($text) (($text -replace "`r`n", "`n") -split "`n" | Where-Object { $_ -notmatch '"evaluationRuntime"' }) -join "`n" }
    $existing = [IO.File]::ReadAllText($fixtureFull)
    Write-Check ((& $strip $existing) -ceq (& $strip $fixtureText)) "fixture 与当前 DLL 原生结果一致：$fixtureFull"
    # worldPoint 专项 职责：逐样本指出被改写或缺失的坐标，十进制与位模式分别对照 DLL 结果；边界：整体文本比较仍是最终判据。
    try {
        $existingPoints = Read-WorldPointKeys $existing
        $pointBad = [Collections.Generic.List[string]]::new()
        if ($existingPoints.Count -ne $generatedPoints.Count) { $pointBad.Add("样本数 $($existingPoints.Count)≠$($generatedPoints.Count)") }
        for ($i = 0; $i -lt [Math]::Min($existingPoints.Count, $generatedPoints.Count); $i++) {
            $want = $generatedPoints[$i]; $have = $existingPoints[$i]
            if ($have.key -ne $want.key -or $have.decimal -ne $want.recorded -or $have.recorded -ne $want.recorded) {
                $pointBad.Add("#$i $($want.key) 期望 $($want.recorded) 实际 key=[$($have.key)] decimal=$($have.decimal) bits=$($have.recorded)")
            }
        }
        Write-Check ($pointBad.Count -eq 0) "fixture worldPointSamples 与 DLL 求值逐样本一致（差异 $($pointBad.Count)）"
        foreach ($line in $pointBad | Select-Object -First 5) { Write-Host "    $line" }
    } catch {
        Write-Check $false "fixture worldPointSamples 无法解析：$($_.Exception.Message)"
    }
} else {
    Write-Check $false "fixture 不存在：$fixtureFull（使用 -UpdateFixture 生成）"
}

# ---------- 3. 游戏 Area 条件检查 ----------
# New-Area 职责：构造与 blueprint.Area JSON 同名字段的样本；边界：只用于脚本内存样本。
function New-Area([int]$index, [int]$parent, [int]$segments, [int]$offsetY, [int]$width, [int]$height, [int]$tropicAnchor = 0, [int]$offsetX = 0) {
    return [pscustomobject]@{
        index = $index; parentIndex = $parent; tropicAnchor = $tropicAnchor; areaSegments = $segments
        anchorLocalOffsetX = $offsetX; anchorLocalOffsetY = $offsetY; width = $width; height = $height
    }
}

# Test-AreaStructure 职责：确认 Area 序列满足游戏遍历顺序前提；使用：游戏调用之前；
# 边界：这是脚本结构检查而非游戏条件；违反时游戏会读取未计算的父区域框，结果无意义。
function Test-AreaStructure($areas, [int]$primary) {
    $count = @($areas).Count
    if ($count -lt 1) { return '区域为空' }
    if ($count -gt $MaxPasteAreas) { return "区域数 $count 超过游戏粘贴工具上限 $MaxPasteAreas" }
    if ($primary -lt 0 -or $primary -ge $count) { return "primaryAreaIndex=$primary 越界" }
    for ($i = 0; $i -lt $count; $i++) {
        $area = $areas[$i]
        if ([int]$area.index -ne $i) { return "区域 $i 的 index=$($area.index) 与序号不一致" }
        if ([int]$area.width -lt 1 -or [int]$area.height -lt 1 -or [int]$area.areaSegments -lt 1) { return "区域 $i 尺寸或 areaSegments 非正" }
        if ($i -eq $primary) { continue }
        $parent = [int]$area.parentIndex
        # 游戏先处理主区域，再按 primary-1..0 和 primary+1..n-1 顺序处理子区域。
        $ordered = if ($i -lt $primary) { $parent -gt $i -and $parent -le $primary } else { $parent -lt $i -and $parent -ge $primary }
        if (-not $ordered) { return "区域 $i 的 parentIndex=$parent 不在游戏遍历顺序之前" }
    }
    return $null
}

# Invoke-GameAreaConditions 职责：以 yaw=0、经度 0 的主区域起点调用游戏 GenerateAreaGratBoxByBPData；
# 使用：样本和文档检查；边界：返回每个 Area 的 IntVector4(x 跨纬带, y 越极/超圈宽, z 经度错位, w 段数比例)。
function Invoke-GameAreaConditions($areas, [int]$primary, [int]$rootLatitude, [int]$segment) {
    $data = [Activator]::CreateInstance($tBlueprintData)
    $native = [Array]::CreateInstance($tBlueprintArea, @($areas).Count)
    for ($i = 0; $i -lt @($areas).Count; $i++) {
        $source = $areas[$i]
        $area = [Activator]::CreateInstance($tBlueprintArea)
        foreach ($name in 'index', 'parentIndex', 'tropicAnchor', 'areaSegments', 'anchorLocalOffsetX', 'anchorLocalOffsetY', 'width', 'height') {
            $area.$name = [int]$source.$name
        }
        $native[$i] = $area
    }
    $data.areas = $native
    $data.primaryAreaIdx = $primary
    $boxes = [Array]::CreateInstance($tGratBox, $MaxPasteAreas)
    $conditions = [Array]::CreateInstance($tIntVector4, $MaxPasteAreas)
    $dots = [Array]::CreateInstance($tVector3, 1)
    $dots[0] = $mGetDir.Invoke($null, @([single]0, (Get-GridLatitudeRad $rootLatitude $segment)))
    $returned = [int]$mGenerate.Invoke($null, @($data, $boxes, $conditions, $dots, 1, [single]0, $segment))
    $result = for ($i = 0; $i -lt @($areas).Count; $i++) {
        $c = $conditions[$i]
        [pscustomobject]@{ area = $i; x = $c.x; y = $c.y; z = $c.z; w = $c.w }
    }
    return [pscustomobject]@{ returned = $returned; conditions = @($result) }
}

# Format-Conditions 职责：输出可读的四类条件；边界：仅格式化。
function Format-Conditions($conditions) {
    return (@($conditions | ForEach-Object { "a$($_.area)=($($_.x),$($_.y),$($_.z),$($_.w))" }) -join ' ')
}

# 样本 职责：用 segment200 原生纬带构造正例和四类反例；边界：区域内无建筑，只检验 Area 元数据。
$eq = 5 * 200       # 赤道纬带一圈细格数
$s160 = 5 * 160
$baseThree = {
    # 南 160 纬带 / 主区域赤道 200 纬带 / 北 160 纬带 / 北 120 纬带，根原点为主区域南缘 -80。
    @(
        (New-Area 0 1 160 -50 $s160 50),
        (New-Area 1 -1 200 0 $eq 161),
        (New-Area 2 1 160 161 $s160 50),
        (New-Area 3 2 120 50 600 25)
    )
}
$samples = [Collections.Generic.List[object]]::new()
$samples.Add(@{ name = '单区域赤道整圈宽度边界'; root = -5; primary = 0; areas = @((New-Area 0 -1 200 0 $eq 11)); expect = $null })
$samples.Add(@{ name = '四区域主区域居中跨三类纬带'; root = -80; primary = 1; areas = (& $baseThree); expect = $null })
$samples.Add(@{ name = '三区域主区域在南端'; root = 81; primary = 0; areas = @((New-Area 0 -1 160 0 $s160 50), (New-Area 1 0 120 50 600 25), (New-Area 2 1 100 25 500 25)); expect = $null })
$samples.Add(@{ name = '反例-单区域跨纬带'; root = 70; primary = 0; areas = @((New-Area 0 -1 200 0 100 20)); expect = @{ area = 0; components = @('x') } })
$samples.Add(@{ name = '反例-单区域圈宽溢出'; root = -5; primary = 0; areas = @((New-Area 0 -1 200 0 ($eq + 1) 11)); expect = @{ area = 0; components = @('y') } })
$samples.Add(@{ name = '反例-越过极点'; root = 246; primary = 0; areas = @((New-Area 0 -1 4 0 10 6)); expect = @{ area = 0; components = @('y') } })
$wrongSegments = & $baseThree; $wrongSegments[2].areaSegments = 120
$samples.Add(@{ name = '反例-子区域 segments 比例错'; root = -80; primary = 1; areas = $wrongSegments; expect = @{ area = 2; components = @('w') } })
$misaligned = & $baseThree; $misaligned[2].tropicAnchor = 1
$samples.Add(@{ name = '反例-子区域经度错位'; root = -80; primary = 1; areas = $misaligned; expect = @{ area = 2; components = @('z') } })
$wrongOffset = & $baseThree; $wrongOffset[2].anchorLocalOffsetY = 160
$samples.Add(@{ name = '反例-子区域纬向错位'; root = -80; primary = 1; areas = $wrongOffset; expect = @{ area = 2; components = @('x', 'w') } })
$childCross = & $baseThree; $childCross[2].height = 51
$samples.Add(@{ name = '反例-子区域高度跨纬带'; root = -80; primary = 1; areas = $childCross; expect = @{ area = 2; components = @('x') } })

$acceptPassed = 0; $acceptTotal = 0; $rejectPassed = 0; $rejectTotal = 0
foreach ($sample in $samples) {
    $structure = Test-AreaStructure $sample.areas $sample.primary
    if ($structure) {
        Write-Check $false "样本 $($sample.name) 结构无效：$structure"
        continue
    }
    $run = Invoke-GameAreaConditions $sample.areas $sample.primary $sample.root $FullTableSegment
    $text = "root=$($sample.root) primary=$($sample.primary) returned=$($run.returned) $(Format-Conditions $run.conditions)"
    if ($null -eq $sample.expect) {
        $acceptTotal++
        $ok = $run.returned -eq @($sample.areas).Count -and -not @($run.conditions | Where-Object { $_.x -or $_.y -or $_.z -or $_.w })
        if ($ok) { $acceptPassed++ }
        Write-Check $ok "接受样本 $($sample.name)：$text"
    } else {
        $rejectTotal++
        $target = $run.conditions[$sample.expect.area]
        $ok = -not @($sample.expect.components | Where-Object { [int]$target.$_ -le 0 })
        if ($ok) { $rejectPassed++ }
        Write-Check $ok "拒绝样本 $($sample.name)（期望 a$($sample.expect.area).$($sample.expect.components -join '+')>0）：$text"
    }
}
Write-Host "样本汇总：接受通过 $acceptPassed/$acceptTotal，拒绝命中 $rejectPassed/$rejectTotal"

# ---------- 4. 可选：核对仓库导出的蓝图文档 ----------
$documentFiles = @()
if ($BlueprintDocumentPath) {
    if (Test-Path -LiteralPath $BlueprintDocumentPath -PathType Container) {
        $documentFiles = @(Get-ChildItem -LiteralPath $BlueprintDocumentPath -Filter '*.json' -File | Sort-Object Name)
        if ($documentFiles.Count -eq 0) { Write-Check $false "目录中没有 JSON 文档：$BlueprintDocumentPath" }
    } elseif (Test-Path -LiteralPath $BlueprintDocumentPath -PathType Leaf) {
        $documentFiles = @(Get-Item -LiteralPath $BlueprintDocumentPath)
    } else {
        Write-Check $false "文档路径不存在：$BlueprintDocumentPath"
    }
}
$documentPassed = 0
foreach ($file in $documentFiles) {
    $label = "文档 $($file.Name)"
    try {
        $json = Get-Content -LiteralPath $file.FullName -Raw | ConvertFrom-Json
        $document = Get-OptionalProperty $json 'document'
        $plan = Get-OptionalProperty $json 'surfacePlan'
        if ($null -eq $document) { $document = $json }
        $areas = @(Get-OptionalProperty $document 'areas')
        $primary = [int](Get-OptionalProperty $document 'primaryAreaIndex')
        $segment = Get-OptionalProperty $plan 'gridSegment'
        $segment = if ($null -eq $segment) { $FullTableSegment } else { [int]$segment }
        $root = if ($null -ne $RootLatitudeGrid) { [int]$RootLatitudeGrid } else { Get-OptionalProperty $plan 'anchorLatitudeGrid' }
        if ($null -eq $root) { throw '缺少 surfacePlan.anchorLatitudeGrid，且未提供 -RootLatitudeGrid' }
        $root = [int]$root
        if ($segment -ne $FullTableSegment) { throw "gridSegment=$segment 不受支持：游戏 float API 吸附默认 200，结论不可用" }
        $planPrimary = Get-OptionalProperty $plan 'primaryAreaIndex'
        if ($null -ne $planPrimary -and [int]$planPrimary -ne $primary) { throw "surfacePlan.primaryAreaIndex=$planPrimary 与 document.primaryAreaIndex=$primary 不一致" }
        $structure = Test-AreaStructure $areas $primary
        if ($structure) { throw "结构无效：$structure" }
        # 计划一致性 职责：用父链与 anchorLocalOffsetY 还原各区域南缘，并与 surfacePlan.bands 对照；边界：bands 缺失时跳过。
        $bandsPlan = @(Get-OptionalProperty $plan 'bands')
        if ($bandsPlan.Count -gt 0 -and $null -ne $bandsPlan[0]) {
            if ($bandsPlan.Count -ne $areas.Count) { throw "surfacePlan.bands 数量 $($bandsPlan.Count) 与 areas 数量 $($areas.Count) 不一致" }
            $low = @{}; $low[$primary] = $root
            # 与游戏相同顺序：primary-1..0，再 primary+1..n-1；PowerShell 范围运算会反向，故用显式循环。
            for ($i = $primary - 1; $i -ge 0; $i--) { $low[$i] = $low[[int]$areas[$i].parentIndex] + [int]$areas[$i].anchorLocalOffsetY }
            for ($i = $primary + 1; $i -lt $areas.Count; $i++) { $low[$i] = $low[[int]$areas[$i].parentIndex] + [int]$areas[$i].anchorLocalOffsetY }
            for ($i = 0; $i -lt $areas.Count; $i++) {
                $band = $bandsPlan[$i]
                $high = $low[$i] + [int]$areas[$i].height - 1
                if ($low[$i] -ne [int]$band.minLatitudeGrid -or $high -ne [int]$band.maxLatitudeGrid) {
                    throw "区域 $i 由父链还原的纬向格 [$($low[$i]),$high] 与 surfacePlan.bands [$($band.minLatitudeGrid),$($band.maxLatitudeGrid)] 不一致"
                }
            }
        }
        $run = Invoke-GameAreaConditions $areas $primary $root $segment
        $bad = @($run.conditions | Where-Object { $_.x -or $_.y -or $_.z -or $_.w })
        $ok = $run.returned -eq $areas.Count -and $bad.Count -eq 0
        if ($ok) { $documentPassed++ }
        Write-Check $ok "$label PrimaryAreaIndex=$primary root=$root areas=$($areas.Count) returned=$($run.returned) $(Format-Conditions $run.conditions)"
        # 提示：游戏 SnapTropic 依赖 areaSegments 与真实纬圈段数一致；此处只提示，不计入游戏四类条件。
        $rootCount = Get-CountInt $root $segment
        if ([int]$areas[$primary].areaSegments -ne $rootCount) {
            Write-Host "  [WARN] 主区域 areaSegments=$($areas[$primary].areaSegments) 与根纬向格 $root 的游戏段数 $rootCount 不同"
        }
        # 生成器坐标交叉核对 职责：对导出证据中的每条首端原始坐标直接调用 DLL 并比较 Go 结果。
        # 使用：DSBOX_SURFACE_EVIDENCE_DIR 导出的单 Area 文档；边界：yaw=0、半径200，不包含 RefreshBuildPreview 后续分拣器端口修正与 Physics。
        $world = Get-OptionalProperty $json 'worldPositions'
        if ($null -ne $world) {
            if ($areas.Count -ne 1) { throw '逐建筑世界坐标交叉核对目前仅支持单 Area' }
            $nativeDocument = [Text.Json.JsonDocument]::Parse([IO.File]::ReadAllText($file.FullName))
            try {
                $rawBuildings = $nativeDocument.RootElement.GetProperty('document').GetProperty('buildings')
                $rawWorld = $nativeDocument.RootElement.GetProperty('worldPositions')
                if ($rawBuildings.GetArrayLength() -ne $rawWorld.GetArrayLength()) { throw '世界坐标证据条数与建筑条数不一致' }
                $rootRad = Get-GridLatitudeRad $root $segment
                $lonSegments = Get-CountFloat $rootRad $segment
                $lonStep = [single]$mLonRadPerGrid.Invoke($null, @($lonSegments, $segment))
                $latStep = Get-LatRadPerGrid $segment
                $mismatches = 0
                for ($i = 0; $i -lt $rawBuildings.GetArrayLength(); $i++) {
                    $offset = $rawBuildings[$i].GetProperty('localOffset')
                    $lng = [single]($offset.GetProperty('x').GetSingle() * $lonStep)
                    $lat = [single]($rootRad + [single]($offset.GetProperty('y').GetSingle() * $latStep))
                    $radius = [single]([single]([single]($offset.GetProperty('z').GetSingle() * $heightStep) + $heightBias) + $planetRadius)
                    $dir = $mGetDir.Invoke($null, @($lng, $lat))
                    $position = $mVectorScale.Invoke($null, @($dir, $radius))
                    foreach ($axis in 'x', 'y', 'z') {
                        $actual = $rawWorld[$i].GetProperty($axis).GetSingle()
                        $ulp = [Math]::Abs([long][BitConverter]::SingleToInt32Bits($actual) - [long][BitConverter]::SingleToInt32Bits([single]$position.$axis))
                        if ($ulp -gt 1) {
                            $mismatches++
                            if ($mismatches -le 3) { Write-Host "    建筑 $i 轴 $axis：Go=$actual DLL=$($position.$axis) 差异=$ulp ULP" }
                        }
                    }
                }
                Write-Check ($mismatches -eq 0) "$label 首端原始世界坐标 $($rawBuildings.GetArrayLength()) 条与 DLL 一致（容差1 ULP，差异 $mismatches）"
            } finally {
                $nativeDocument.Dispose()
            }
        }
    } catch {
        Write-Check $false "$label 无法检查：$($_.Exception.Message)"
    }
}
if ($documentFiles.Count -gt 0) {
    Write-Host "文档汇总：通过 $documentPassed/$($documentFiles.Count)"
}

if ($script:Failures -gt 0) {
    Write-Host "结果：失败 $($script:Failures) 项"
    exit 1
}
Write-Host '结果：全部通过'
exit 0
