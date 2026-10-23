<#
.SYNOPSIS
    启动 / 停止 / 查看 CodeFever Next 本地环境。

.EXAMPLE
    pwsh -File scripts/run-local.ps1 up
    pwsh -File scripts/run-local.ps1 status
    pwsh -File scripts/run-local.ps1 logs
    pwsh -File scripts/run-local.ps1 down
#>
[CmdletBinding()]
param(
    [ValidateSet('up', 'down', 'restart', 'status', 'logs', 'ps')]
    [string] $Action = 'up'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$compose = @('compose', '-f', 'docker-compose.yml')
$httpPort = if ($env:CODEFEVER_HTTP_PORT) { $env:CODEFEVER_HTTP_PORT } else { '9080' }
$sshPort  = if ($env:CODEFEVER_SSH_PORT)  { $env:CODEFEVER_SSH_PORT }  else { '2222' }
$httpUrl = "http://localhost:$httpPort"
$sshHint = "ssh -p $sshPort git@localhost"

function Write-Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "    [OK] $msg" -ForegroundColor Green }

switch ($Action) {
    'up' {
        Write-Step '启动服务'
        docker @compose up -d
        if ($LASTEXITCODE -ne 0) { throw '启动失败' }

        Write-Step '等待数据库与应用就绪'
        $ready = $false
        for ($i = 0; $i -lt 60; $i++) {
            Start-Sleep -Seconds 3
            $code = (docker @compose ps --format '{{.Name}} {{.Status}}' 2>$null) -join "`n"
            if ($code -match 'codefever-next-app.*\(healthy\)') { $ready = $true; break }
            if ($code -match 'codefever-next-app.*Up') { $ready = $true; break }
        }

        docker @compose ps
        Write-Host ''
        if ($ready) {
            Write-Ok "服务已启动"
            Write-Host "    Web UI : $httpUrl" -ForegroundColor Green
            Write-Host "    登录   : root@codefever.cn / 123456" -ForegroundColor Green
            Write-Host "    Git SSH: $sshHint" -ForegroundColor Green
        } else {
            Write-Warn2 '服务仍在初始化, 用 `pwsh -File scripts/run-local.ps1 logs` 查看日志'
        }
    }
    'down' {
        Write-Step '停止服务 (保留数据卷)'
        docker @compose down
    }
    'restart' {
        Write-Step '重启服务'
        docker @compose restart
    }
    'status' {
        docker @compose ps
        Write-Host ''
        Write-Step '健康检查'
        try {
            $r = Invoke-WebRequest -Uri $httpUrl -UseBasicParsing -TimeoutSec 15
            Write-Ok "HTTP $($r.StatusCode)  ($($r.RawContentLength) bytes)"
        } catch {
            Write-Warn2 "HTTP 请求失败: $($_.Exception.Message)"
        }
    }
    'logs' {
        docker @compose logs -f --tail 200
    }
    'ps' {
        docker @compose ps
    }
}
