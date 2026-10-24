<#
.SYNOPSIS
    CodeFever Next 本地质量门禁（编译 / 格式 / vet / 单测 / Compose 校验 / 可选镜像构建）。

.DESCRIPTION
    这是维护者与 CI 共用的最小门禁，不依赖运行中的数据库、浏览器或外部服务。
    契约回归、截图比对、写操作冒烟属于**有前置条件**的深度验证，见
    docs/IMPLEMENTATION_STATUS.md 与 backend/cmd/ 下的工具，不在本脚本内自动执行。

.EXAMPLE
    pwsh -File scripts/verify.ps1
    pwsh -File scripts/verify.ps1 -BuildImage
#>
[CmdletBinding()]
param(
    [switch]$BuildImage
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$results = New-Object System.Collections.Generic.List[object]

function Step([string]$name, [scriptblock]$body) {
    Write-Host ''
    Write-Host "=== $name ===" -ForegroundColor Cyan
    try {
        & $body
        $results.Add([pscustomobject]@{ Step = $name; Result = 'OK'; Detail = '' })
    } catch {
        Write-Host ("FAILED: " + $_.Exception.Message) -ForegroundColor Red
        $results.Add([pscustomobject]@{ Step = $name; Result = 'FAIL'; Detail = $_.Exception.Message })
    }
}

function Assert-True([bool]$cond, [string]$msg) {
    if (-not $cond) { throw $msg }
}

$env:GOFLAGS = '-mod=mod'

Push-Location backend
try {
    Step 'gofmt' {
        $unformatted = gofmt -l .
        Assert-True ($LASTEXITCODE -eq 0) 'gofmt 执行失败'
        Assert-True (-not $unformatted) ("以下文件未格式化: " + ($unformatted -join ', '))
    }

    Step 'go build' {
        go build ./...
        Assert-True ($LASTEXITCODE -eq 0) 'go build 失败'
    }

    Step 'go build (linux)' {
        $oldGOOS = $env:GOOS
        $oldGOARCH = $env:GOARCH
        $oldCGO = $env:CGO_ENABLED
        try {
            $env:GOOS = 'linux'
            $env:GOARCH = 'amd64'
            $env:CGO_ENABLED = '0'
            go build ./...
            Assert-True ($LASTEXITCODE -eq 0) 'linux 交叉编译失败'
        } finally {
            $env:GOOS = $oldGOOS
            $env:GOARCH = $oldGOARCH
            $env:CGO_ENABLED = $oldCGO
        }
    }

    Step 'go vet' {
        go vet ./...
        Assert-True ($LASTEXITCODE -eq 0) 'go vet 失败'
    }

    Step 'go test' {
        go test ./internal/...
        Assert-True ($LASTEXITCODE -eq 0) 'go test 失败'
    }
} finally {
    Pop-Location
}

Step 'docker compose config' {
    docker compose config | Out-Null
    Assert-True ($LASTEXITCODE -eq 0) 'docker compose config 失败'
}

if ($BuildImage) {
    Step 'docker build' {
        docker build -t codefever-next:local .
        Assert-True ($LASTEXITCODE -eq 0) 'docker build 失败'
    }
}

Write-Host ''
$failed = $results | Where-Object { $_.Result -ne 'OK' }
$results | Format-Table -AutoSize
if ($failed) {
    Write-Host ("有 " + $failed.Count + " 项失败") -ForegroundColor Red
    exit 1
}
Write-Host '全部通过' -ForegroundColor Green
