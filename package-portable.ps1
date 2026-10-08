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

$previousGoCache = $env:GOCACHE
$previousGoTmp = $env:GOTMPDIR
try {
New-Item -ItemType Directory -Force -Path $payloadDir, $distDir, $goCacheDir, $goTmpDir | Out-Null
$env:GOCACHE = $goCacheDir
$env:GOTMPDIR = $goTmpDir

Push-Location (Join-Path $projectRoot "components\autobhop")
try {
    go test -count=1 -timeout=120s ./...
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
    go test -count=1 -timeout=120s ./...
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
    go test -count=1 -timeout=120s ./...
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

# The host uses this build-version manifest to avoid re-reading fixed payloads on warm starts.
$manifest = [ordered]@{}
Get-ChildItem -LiteralPath $payloadDir -Recurse -File | Sort-Object FullName | ForEach-Object {
    if ($_.Name -ne "startup-manifest.json") {
        $relative = $_.FullName.Substring($payloadDir.Length + 1).Replace('\', '/')
        $manifest[$relative] = [ordered]@{size=$_.Length; sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()}
    }
}
$manifestJson = $manifest | ConvertTo-Json -Depth 4 -Compress
[IO.File]::WriteAllText((Join-Path $payloadDir "startup-manifest.json"), $manifestJson, [Text.UTF8Encoding]::new($false))

Push-Location $projectRoot
try {
    windres.exe .\app.rc -O coff -o .\app.syso
    Assert-NativeSuccess "Resource build"
    go test -count=1 -timeout=120s ./...
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

# Package only release-owned files; never copy dist/data or personal configuration.
$portableDir = Join-Path $tmpDir "portable-stage"
New-Item -ItemType Directory -Force -Path $portableDir | Out-Null
$portableExe = Join-Path $portableDir "L4D2_MIX.exe"
$portableReadme = Join-Path $portableDir "README.txt"
Copy-Item -LiteralPath $builtExe -Destination $portableExe -Force
$portableText = @"
L4D2 MIX - portable optimized build

Run L4D2_MIX.exe. Windows requests administrator permission.
Only the executable is required; embedded components are extracted automatically.
Your configuration is created in the data directory beside the executable.
Keep that data directory when upgrading. It is deliberately excluded from this ZIP.
Default page starts first; other pages load in the background.

Optional startup tracing: L4D2_MIX_TRACE_STARTUP=1
Optional full payload verification: L4D2_MIX_VERIFY_PAYLOAD=1
"@
[IO.File]::WriteAllText($portableReadme, $portableText, [Text.UTF8Encoding]::new($false))
$portableZip = Join-Path $distDir "L4D2_MIX-portable.zip"
Compress-Archive -LiteralPath $portableExe,$portableReadme -DestinationPath $portableZip -Force
$hash = (Get-FileHash -LiteralPath $portableZip -Algorithm SHA256).Hash.ToLowerInvariant()
[IO.File]::WriteAllText(($portableZip + ".sha256"), "$hash  L4D2_MIX-portable.zip`n", [Text.UTF8Encoding]::new($false))
Write-Host "Portable archive created: $portableZip"
} finally {
    $env:GOCACHE = $previousGoCache
    $env:GOTMPDIR = $previousGoTmp
}
