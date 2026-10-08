param(
    [ValidateRange(1, 10)][int]$Rounds = 2,
    [switch]$Build,
    [string]$OutputPath = ""
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
if (!$OutputPath) { $OutputPath = Join-Path $projectRoot '.tmp/background-verification.json' }
$OutputPath = [IO.Path]::GetFullPath($OutputPath)
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutputPath) | Out-Null
$records = [Collections.Generic.List[object]]::new()
$artifacts = [Collections.Generic.List[object]]::new()
$trees = @($projectRoot)
$success = $false

function Invoke-HiddenCommand([string]$Directory, [string]$File, [string[]]$Arguments) {
    $info = [Diagnostics.ProcessStartInfo]::new()
    $info.FileName = $File
    $info.Arguments = ($Arguments | ForEach-Object { '"' + $_.Replace('"', '\"') + '"' }) -join ' '
    $info.WorkingDirectory = $Directory
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    $info.StandardOutputEncoding = [Text.UTF8Encoding]::new($false)
    $info.StandardErrorEncoding = [Text.UTF8Encoding]::new($false)
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $info
    $timer = [Diagnostics.Stopwatch]::StartNew()
    try {
        if (!$process.Start()) { throw "Failed to start $File" }
        $stdout = $process.StandardOutput.ReadToEndAsync()
        $stderr = $process.StandardError.ReadToEndAsync()
        if (!$process.WaitForExit(600000)) { $process.Kill(); throw "Command timeout: $File $($info.Arguments)" }
        $process.WaitForExit()
        $result = [ordered]@{cwd=$Directory; executable=$File; arguments=$Arguments; stdout=$stdout.Result; stderr=$stderr.Result; exit=$process.ExitCode; milliseconds=$timer.ElapsedMilliseconds}
        $records.Add($result)
        Write-Host "$Directory :: $File $($Arguments -join ' ') => $($result.exit)"
        return $result
    } finally { $process.Dispose() }
}

function Assert-Success($Result) {
    if ($Result.exit -ne 0) { throw "Command failed: $($Result.stderr) $($Result.stdout)" }
}

function Assert-KnownNativePointerDiagnostics($Result, [int]$ExpectedCount) {
    if ($Result.exit -ne 1) { throw "Unexpected full vet exit: $($Result.exit)" }
    $count = 0
    foreach ($line in (($Result.stdout + $Result.stderr) -split "`r?`n")) {
        if (!$line.Trim() -or $line.StartsWith('#')) { continue }
        if ($line -notmatch '^(.+\.go):(\d+):(\d+): possible misuse of unsafe\.Pointer$') { throw "New vet diagnostic: $line" }
        $source = Join-Path $Result.cwd $Matches[1]
        $sourceLine = [IO.File]::ReadAllLines($source)[[int]$Matches[2] - 1].Trim()
        # Only the existing Win32 message ABI conversions are accepted; all
        # other analyzers still run, and the full literal diagnostics are saved.
        if ($sourceLine -notmatch '^(drawOwnerButton|drawButton|drawThemedButton)\(\(\*drawItemStruct\)\(unsafe\.Pointer\(l[Pp]aram\)\)\)$' -and
            $sourceLine -ne 'info := (*minMaxInfo)(unsafe.Pointer(lParam))') { throw "Unreviewed native pointer conversion: $sourceLine" }
        $count++
    }
    if ($count -ne $ExpectedCount) { throw "Expected $ExpectedCount ABI diagnostics; got $count" }
    $Result['classification'] = 'reviewed Win32 lParam ABI diagnostics, not zero-warning vet'
}

function Assert-ProjectLayout {
    foreach ($module in @('', 'components/autobhop', 'components/rowfilter', 'components/modjoin')) {
        $directory = if ($module) { Join-Path $projectRoot $module } else { $projectRoot }
        if (!(Test-Path -LiteralPath (Join-Path $directory 'go.mod'))) { throw "Missing project module: $directory" }
    }
    Write-Host 'Single project layout: host and three component modules verified.'
}

function Test-PortableArchive([string]$Tree) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $exe = Join-Path $Tree 'dist/L4D2_MIX.exe'
    $zip = Join-Path $Tree 'dist/L4D2_MIX-portable.zip'
    $hash = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
    $sidecar = [IO.File]::ReadAllText($zip + '.sha256').Trim()
    if ($sidecar -ne "$hash  L4D2_MIX-portable.zip") { throw "ZIP checksum mismatch: $zip" }
    $archive = [IO.Compression.ZipFile]::OpenRead($zip)
    try {
        $names = @($archive.Entries | ForEach-Object FullName | Sort-Object)
        if (($names -join '|') -ne 'L4D2_MIX.exe|README.txt') { throw "Unexpected portable entries: $names" }
        $entry = $archive.GetEntry('L4D2_MIX.exe')
        $stream = $entry.Open()
        $sha = [Security.Cryptography.SHA256]::Create()
        try { $embeddedHash = [BitConverter]::ToString($sha.ComputeHash($stream)).Replace('-','').ToLowerInvariant() }
        finally { $sha.Dispose(); $stream.Dispose() }
        $exeHash = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($embeddedHash -ne $exeHash) { throw "ZIP executable differs from built executable: $zip" }
        $artifacts.Add([ordered]@{exe=$exe; exe_sha256=$exeHash; zip=$zip; zip_sha256=$hash; entries=$names})
    } finally { $archive.Dispose() }
}

function Assert-PowerShellSyntax([string]$Tree) {
    $files = @(Get-ChildItem -LiteralPath $Tree -File -Filter '*.ps1')
    $files += @(Get-ChildItem -LiteralPath (Join-Path $Tree 'scripts'),(Join-Path $Tree 'payload/runtime') -Recurse -File -Filter '*.ps1')
    foreach ($file in $files) {
        $parseErrors = $null
        [Management.Automation.Language.Parser]::ParseFile($file.FullName, [ref]$null, [ref]$parseErrors) | Out-Null
        if ($parseErrors.Count -gt 0) { throw "PowerShell syntax errors: $($file.FullName) $parseErrors" }
    }
    $records.Add([ordered]@{check='PowerShell ParseFile (parse only, no script execution)'; cwd=$Tree; files=@($files | ForEach-Object FullName); exit=0})
}

function Assert-PayloadManifest([string]$Tree) {
    $payload = Join-Path $Tree 'payload'
    $manifest = [IO.File]::ReadAllText((Join-Path $payload 'startup-manifest.json')) | ConvertFrom-Json
    $count = 0
    foreach ($entry in $manifest.PSObject.Properties) {
        $path = [IO.Path]::GetFullPath((Join-Path $payload $entry.Name))
        if (!$path.StartsWith(($payload + [IO.Path]::DirectorySeparatorChar), [StringComparison]::OrdinalIgnoreCase)) { throw "Invalid manifest path: $($entry.Name)" }
        $file = Get-Item -LiteralPath $path
        $hash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($file.Length -ne $entry.Value.size -or $hash -ne $entry.Value.sha256) { throw "Payload manifest mismatch: $path" }
        $count++
    }
    $actualCount = @(Get-ChildItem -LiteralPath $payload -Recurse -File | Where-Object Name -ne 'startup-manifest.json').Count
    if ($count -ne $actualCount) { throw "Incomplete payload manifest: $count / $actualCount" }
    $records.Add([ordered]@{check='Payload manifest SHA-256 and size for all embedded files'; cwd=$Tree; count=$count; exit=0})
}

try {
    Assert-ProjectLayout
    foreach ($tree in $trees) {
        Assert-PowerShellSyntax $tree
        if ($Build) {
            $buildResult = Invoke-HiddenCommand $tree 'powershell.exe' @('-NoProfile','-ExecutionPolicy','Bypass','-File',(Join-Path $tree 'package-portable.ps1'))
            Assert-Success $buildResult
            Test-PortableArchive $tree
        }
        Assert-PayloadManifest $tree
        foreach ($round in 1..$Rounds) {
            foreach ($module in @('', 'components/autobhop', 'components/rowfilter', 'components/modjoin')) {
                $directory = if ($module) { Join-Path $tree $module } else { $tree }
                foreach ($arguments in @(@('test','-count=1','-timeout=120s','./...'), @('test','-race','-count=2','-timeout=120s','./...'), @('vet','-unsafeptr=false','./...'))) {
                    $result = Invoke-HiddenCommand $directory 'go' $arguments
                    $result['round'] = $round
                    Assert-Success $result
                }
                $vet = Invoke-HiddenCommand $directory 'go' @('vet','./...')
                $expected = if ($module -eq '' -or $module -eq 'components/modjoin') { 2 } else { 1 }
                Assert-KnownNativePointerDiagnostics $vet $expected
                $sourceFiles = if (!$module) { @(Get-ChildItem -LiteralPath $directory -Filter '*.go' -File) } else { @(Get-ChildItem -LiteralPath $directory -Filter '*.go' -File -Recurse) }
                $format = Invoke-HiddenCommand $directory 'gofmt' (@('-l') + @($sourceFiles | ForEach-Object FullName))
                Assert-Success $format
                if ($format.stdout.Trim()) { throw "Unformatted source files: $($format.stdout)" }
            }
        }
    }
    Assert-ProjectLayout
    $success = $true
} finally {
    $report = [ordered]@{success=$success; rounds=$Rounds; visible_ui_started=$false; notes='Hidden HWND tests only. Desktop first-frame compositing and real game integration not exercised.'; commands=@($records.ToArray()); artifacts=@($artifacts.ToArray())}
    [IO.File]::WriteAllText($OutputPath, ($report | ConvertTo-Json -Depth 12), [Text.UTF8Encoding]::new($false))
    Write-Host "Background verification record: $OutputPath"
}
