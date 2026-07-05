param(
    [switch]$VerifyUI
)

$ErrorActionPreference = "Stop"

$projectRoot = $PSScriptRoot
$payloadDir = Join-Path $projectRoot "payload"
$distDir = Join-Path $projectRoot "dist"
$tmpDir = Join-Path $projectRoot ".tmp"
$goCacheDir = Join-Path $tmpDir "go-cache"
$goTmpDir = Join-Path $tmpDir "go-tmp"

function Assert-NativeSuccess([string]$Step) {
    if ($LASTEXITCODE -ne 0) {
        throw "$Step failed with exit code $LASTEXITCODE"
    }
}

function Assert-CommandAvailable([string]$Command) {
    if (!(Get-Command $Command -ErrorAction SilentlyContinue)) {
        throw "Required command is not available on PATH: $Command"
    }
}

Assert-CommandAvailable "go"
Assert-CommandAvailable "windres.exe"

New-Item -ItemType Directory -Force -Path $payloadDir, $distDir, $goCacheDir, $goTmpDir | Out-Null
$env:GOCACHE = $goCacheDir
$env:GOTMPDIR = $goTmpDir

Push-Location (Join-Path $projectRoot "components\autobhop")
try {
    go test ./...
    Assert-NativeSuccess "Autobhop tests"
    go build -trimpath -ldflags "-H=windowsgui -s -w" `
        -o (Join-Path $payloadDir "L4D2AutobhopVPKW.exe") .
    Assert-NativeSuccess "Autobhop build"
}
finally {
    Pop-Location
}

Push-Location (Join-Path $projectRoot "components\rowfilter")
try {
    go test ./...
    Assert-NativeSuccess "Row filter tests"
    go build -trimpath -ldflags "-H=windowsgui -s -w" `
        -o (Join-Path $payloadDir "L4D2RowFilterManager.exe") .
    Assert-NativeSuccess "Row filter build"
}
finally {
    Pop-Location
}

Push-Location (Join-Path $projectRoot "components\modjoin")
try {
    go test ./...
    Assert-NativeSuccess "MOD join tests"
    go build -trimpath -ldflags "-H=windowsgui -s -w" `
        -o (Join-Path $payloadDir "L4D2ModJoin.exe") .\cmd\l4d2modjoin
    Assert-NativeSuccess "MOD join build"
}
finally {
    Pop-Location
}

$requiredRuntime = @(
    "runtime\matchmaking_probe_loader\L4D2MatchmakingProbeLoader.exe",
    "runtime\matchmaking_row_filter_dll\matchmaking_row_filter.dll",
    "runtime\matchmaking_row_filter_dll\launch_row_filter_early_admin.ps1",
    "runtime\matchmaking_row_filter_dll\load_row_filter_admin.ps1",
    "runtime\matchmaking_row_filter_dll\restore_default_configs.ps1"
)
foreach ($relative in $requiredRuntime) {
    $path = Join-Path $payloadDir $relative
    if (!(Test-Path -LiteralPath $path)) {
        throw "Required in-project runtime file is missing: $path"
    }
}

Push-Location $projectRoot
try {
    windres.exe .\app.rc -O coff -o .\app.syso
    Assert-NativeSuccess "Resource build"
    go test ./...
    Assert-NativeSuccess "Host tests"
    go build -trimpath -ldflags "-H=windowsgui -s -w" -o (Join-Path $distDir "L4D2_MIX.exe") .
    Assert-NativeSuccess "Host build"
}
finally {
    Pop-Location
}

Write-Host "Portable executable created:"
$builtExe = Join-Path $distDir "L4D2_MIX.exe"
Write-Host $builtExe

if ($VerifyUI) {
    $verifyScript = Join-Path $projectRoot "scripts\verify-ui-switch.ps1"
    $outputPath = Join-Path $tmpDir "switch-results.json"
    $screenshotDir = Join-Path $tmpDir "ui-verify"
    if (!(Test-Path -LiteralPath $verifyScript)) {
        throw "UI verification script is missing: $verifyScript"
    }
    Write-Host "Starting UI verification. Approve the UAC prompt if Windows asks for administrator permission."
    $existingPids = @(Get-Process L4D2_MIX -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
    Start-Process -FilePath $builtExe | Out-Null
    Start-Sleep -Seconds 8
    & $verifyScript -OutputPath $outputPath -ScreenshotDir $screenshotDir -ExcludeProcessId $existingPids
    Assert-NativeSuccess "UI switch verification"
    Write-Host "UI verification result:"
    Write-Host $outputPath
}
