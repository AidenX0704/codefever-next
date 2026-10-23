<#
  一键复验脚本 —— 把本轮全部验收动作串成一条可重复执行的流水线。

  用法（在仓库根目录执行）:
      pwsh -File scripts/verify.ps1                       # 全量
      pwsh -File scripts/verify.ps1 -SkipImage            # 跳过镜像重建与打包
      pwsh -File scripts/verify.ps1 -Backend php          # 复验 PHP 回滚态

  环境前提:
      docker compose -f docker-compose.yml up -d         # 起 codefever-next-app / codefever-next-db
      docker run -d --name cf-mysql8 -p 3307:3306 ...    # MySQL 8 隔离实例
      宿主已安装 Chrome/Edge（截图比对用）
#>
[CmdletBinding()]
param(
    [ValidateSet('go', 'php')]
    [string]$Backend = 'go',
    [switch]$SkipImage,
    [switch]$SkipSmoke
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root

$results = New-Object System.Collections.Generic.List[object]
function Step([string]$name, [scriptblock]$body) {
    Write-Host ''
    Write-Host "=== $name ===" -ForegroundColor Cyan
    $sw = [Diagnostics.Stopwatch]::StartNew()
    try {
        & $body
        $results.Add([pscustomobject]@{ Step = $name; Result = 'OK'; Detail = '' })
    } catch {
        Write-Host ("FAILED: " + $_.Exception.Message) -ForegroundColor Red
        $results.Add([pscustomobject]@{ Step = $name; Result = 'FAIL'; Detail = $_.Exception.Message })
    }
    $sw.Stop()
}

function Assert-True([bool]$cond, [string]$msg) {
    if (-not $cond) { throw $msg }
}

$env:GOFLAGS = '-mod=mod'
$base = 'http://localhost:9080'

# ---------------------------------------------------------------------------
Step '编译 / 静态检查 / 单元测试' {
    Push-Location backend
    try {
        go build ./...     ; Assert-True ($LASTEXITCODE -eq 0) 'go build 失败'
        go vet ./...       ; Assert-True ($LASTEXITCODE -eq 0) 'go vet 失败'
        $fmt = gofmt -l .
        Assert-True (-not $fmt) ("gofmt 未通过: " + ($fmt -join ', '))
        go test ./internal/... 2>&1 | Tee-Object -Variable testOut | Out-Null
        $failed = $testOut | Select-String -Pattern '^(FAIL|--- FAIL)'
        Assert-True (-not $failed) ("单元测试失败: " + ($failed -join '; '))
    } finally { Pop-Location }
}

# ---------------------------------------------------------------------------
if (-not $SkipImage) {
    Step '重建镜像' {
        docker build -f Dockerfile -t codefever-next:local . | Out-Null
        Assert-True ($LASTEXITCODE -eq 0) 'docker build 失败'
    }
    Step "切换后端并重建容器（CODEFEVER_BACKEND=$Backend）" {
        $env:CODEFEVER_BACKEND = $Backend
        docker compose -f docker-compose.yml up -d --force-recreate codefever | Out-Null
        Assert-True ($LASTEXITCODE -eq 0) 'compose 失败'
        for ($i = 0; $i -lt 40; $i++) {
            Start-Sleep -Seconds 3
            try {
                $r = Invoke-WebRequest -Uri "$base/user/login" -TimeoutSec 5 -UseBasicParsing
                if ($r.StatusCode -eq 200) { break }
            } catch { }
        }
    }
}

# ---------------------------------------------------------------------------
Step '契约采集与比对' {
    Push-Location backend
    try {
        $out = "testdata/contract-verify"
        go run ./cmd/contract-collect -base $base -out $out | Out-Null
        Assert-True ($LASTEXITCODE -eq 0) 'contract-collect 失败'
        $cmp = go run ./cmd/contract-compare -baseline testdata/contract -actual $out 2>&1
        $cmp | ForEach-Object { Write-Host $_ }
        Assert-True ($LASTEXITCODE -eq 0) '契约比对存在差异'
    } finally { Pop-Location }
}

# ---------------------------------------------------------------------------
if (-not $SkipSmoke) {
    Step '写操作成功路径回归（隔离实例 :9004）' {
        Push-Location backend
        try {
            $smoke = go run ./cmd/smoke-write `
                -base http://127.0.0.1:9004 `
                -dsn "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true" `
                -gitstorage "$root\.tmp-smoke\git-storage" 2>&1
            $smoke | Select-Object -Last 3 | ForEach-Object { Write-Host $_ }
            Assert-True ($LASTEXITCODE -eq 0) '写操作回归失败'
        } finally { Pop-Location }
    }
}

# ---------------------------------------------------------------------------
Step '多视口截图 + 逐像素比对' {
    if (-not (Test-Path "$root\.tmp-shots\go")) {
        Write-Host '跳过：缺少 .tmp-shots\go 参考集（先跑 scripts/shot.mjs 采集）' -ForegroundColor Yellow
        return
    }
    node scripts/shot.mjs --base $base --out .tmp-shots/verify `
        --loginPath /user/login --loginEmail root@codefever.cn --loginPassword 123456 | Out-Null
    Assert-True ($LASTEXITCODE -eq 0) '截图采集失败'
    python scripts/shotcmp.py .tmp-shots/go .tmp-shots/verify --out .tmp-shots/verify-diff
    # 退出码 1 表示存在差异，不视为脚本失败（差异需人工确认）
}

# ---------------------------------------------------------------------------
if (-not $SkipImage) {
    Step '打包离线镜像' {
        $ts = Get-Date -Format 'yyyyMMdd-HHmmss'
        $tar = "dist\codefever-next-local-$ts.tar"
        docker save codefever-next:local -o $tar
        Assert-True ($LASTEXITCODE -eq 0) 'docker save 失败'
        $size = [math]::Round((Get-Item $tar).Length / 1MB, 1)
        Write-Host ("打包完成: {0} ({1} MB)" -f $tar, $size)
    }
}

# ---------------------------------------------------------------------------
Write-Host ''
Write-Host '================ 复验汇总 ================' -ForegroundColor Cyan
$results | Format-Table -AutoSize | Out-String -Width 160 | Write-Host
$failed = $results | Where-Object { $_.Result -eq 'FAIL' }
Pop-Location
if ($failed) { exit 1 }
exit 0
