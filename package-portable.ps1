$ErrorActionPreference = "Stop"

$projectRoot = $PSScriptRoot
$payloadDir = Join-Path $projectRoot "payload"
$distDir = Join-Path $projectRoot "dist"

function Assert-NativeSuccess([string]$Step) {
    if ($LASTEXITCODE -ne 0) {
        throw "$Step failed with exit code $LASTEXITCODE"
    }
}

New-Item -ItemType Directory -Force -Path $payloadDir, $distDir | Out-Null

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
Write-Host (Join-Path $distDir "L4D2_MIX.exe")
