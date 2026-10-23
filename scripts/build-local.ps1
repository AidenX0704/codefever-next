<#
.SYNOPSIS
    构建并打包 CodeFever Next 本地镜像 (从本地源码, 走本地镜像仓库)。

.DESCRIPTION
    1. 确认基础镜像已在本地 (缺失时经 $Registry 镜像仓库拉取并打上本地 tag)
    2. 用 Dockerfile 构建 codefever-next:local
    3. docker save 导出为 tar 归档, 便于离线分发 / 导入其他机器

.EXAMPLE
    pwsh -File scripts/build-local.ps1
    pwsh -File scripts/build-local.ps1 -SkipPackage
#>
[CmdletBinding()]
param(
    [string] $Image     = 'codefever-next:local',
    [string] $Registry  = 'docker.1ms.run',
    [string] $OutDir    = 'dist',
    [switch] $SkipPackage,
    [switch] $NoCache
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

function Write-Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "    [OK] $msg" -ForegroundColor Green }
function Write-Warn2($msg){ Write-Host "    [!!] $msg" -ForegroundColor Yellow }

# 基础镜像: 本地 tag => 镜像仓库路径
$baseImages = @(
    @{ Local = 'webdevops/php-nginx:7.4'; Mirror = "$Registry/webdevops/php-nginx:7.4"; Role = 'runtime+build base' },
    @{ Local = 'golang:1.25-bookworm';    Mirror = "$Registry/library/golang:1.25-bookworm"; Role = 'go builder base' },
    @{ Local = 'mysql:8.0';               Mirror = "$Registry/library/mysql:8.0"; Role = 'database' }
)

Write-Step '检查基础镜像'
foreach ($b in $baseImages) {
    if (docker image inspect $b.Local -f '{{.Id}}' 2>$null) {
        Write-Ok "$($b.Local) 已在本地 ($($b.Role))"
        continue
    }
    Write-Warn2 "$($b.Local) 不在本地, 从 $($b.Mirror) 拉取 ..."
    docker pull $b.Mirror
    if ($LASTEXITCODE -ne 0) { throw "拉取失败: $($b.Mirror)" }
    docker tag $b.Mirror $b.Local
    Write-Ok "$($b.Local) 已就绪"
}

Write-Step "构建镜像 $Image"
$buildArgs = @(
    'build'
    '-f', 'Dockerfile'
    '-t', $Image
    # 基础镜像用上面已打好的**本地 tag**，构建期不直连 Docker Hub。
    '--build-arg', 'BASE_RUNTIME=webdevops/php-nginx:7.4'
    '--build-arg', 'BASE_GO=golang:1.25-bookworm'
)
if ($NoCache) { $buildArgs += '--no-cache' }
$buildArgs += '.'

docker @buildArgs
if ($LASTEXITCODE -ne 0) { throw '镜像构建失败' }

Write-Ok '构建完成'
docker image inspect $Image -f '    ID: {{.Id}}  Size: {{.Size}} bytes'

if ($SkipPackage) {
    Write-Step '已跳过打包 (-SkipPackage)'
    return
}

Write-Step '打包为 tar 归档'
if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Path $OutDir | Out-Null }
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$safeName = ($Image -replace '[:\\/]', '_')
$tarPath = Join-Path $OutDir "$safeName-$stamp.tar"

docker save -o $tarPath $Image
if ($LASTEXITCODE -ne 0) { throw "docker save 失败: $tarPath" }

$sizeMb = [math]::Round((Get-Item $tarPath).Length / 1MB, 1)
Write-Ok "归档文件: $tarPath  ($sizeMb MB)"
Write-Host ''
Write-Host '在其他机器上导入 / Import on another machine:' -ForegroundColor Cyan
Write-Host "    docker load -i $(Split-Path -Leaf $tarPath)"
Write-Host '导入后用 docker-compose.yml 启动 (需把 image 名改成对应名称)。' -ForegroundColor Cyan
