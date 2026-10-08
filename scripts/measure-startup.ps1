param(
    [Parameter(Mandatory=$true)][string]$Exe,
    [Parameter(Mandatory=$true)][string]$OutputPath,
    [Parameter(Mandatory=$true)][string]$FixtureRoot,
    [int]$Runs = 2,
    [switch]$RequireReady
)
$ErrorActionPreference = 'Stop'
Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Collections.Generic;
using System.Text;
public static class StartupProbe {
    public delegate bool EnumProc(IntPtr h, IntPtr p);
    [DllImport("user32.dll")] public static extern bool EnumChildWindows(IntPtr h, EnumProc cb, IntPtr p);
    [DllImport("user32.dll")] public static extern int GetClassName(IntPtr h, StringBuilder s, int n);
    [DllImport("user32.dll")] public static extern IntPtr GetProp(IntPtr h, string s);
    [DllImport("user32.dll")] public static extern int GetWindowLong(IntPtr h, int n);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
    [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr h, uint m, IntPtr w, IntPtr l);
    public static IntPtr Find(IntPtr root, string cls) {
        IntPtr found = IntPtr.Zero;
        EnumChildWindows(root, (h,p) => { var s = new StringBuilder(128); GetClassName(h,s,128); if(s.ToString()==cls) { found=h; return false; } return true; },IntPtr.Zero);
        return found;
    }
    public static int Count(IntPtr root) { int n=0; EnumChildWindows(root,(h,p)=>{n++;return true;},IntPtr.Zero);return n; }
}
"@
$Exe = (Resolve-Path -LiteralPath $Exe).Path
New-Item -ItemType Directory -Force -Path $FixtureRoot | Out-Null
$FixtureRoot = (Resolve-Path -LiteralPath $FixtureRoot).Path
$oldCache = $env:LOCALAPPDATA
$oldRoot = $env:L4D2_MIX_HOST_ROOT
$oldOrigin = $env:L4D2_MIX_START_NS
$results = @()
try {
    $env:LOCALAPPDATA = Join-Path $FixtureRoot 'cache'
    $env:L4D2_MIX_HOST_ROOT = Join-Path $FixtureRoot 'host'
    New-Item -ItemType Directory -Force -Path $env:LOCALAPPDATA,$env:L4D2_MIX_HOST_ROOT | Out-Null
    for ($i=0; $i -lt $Runs; $i++) {
        $sw = [Diagnostics.Stopwatch]::StartNew()
        $env:L4D2_MIX_START_NS = (([DateTime]::UtcNow.Ticks - 621355968000000000) * 100).ToString()
        $p = Start-Process -FilePath $Exe -WindowStyle Hidden -PassThru
        $row = [ordered]@{Run=$i; Exe=$Exe; HostMs=$null; BhopMs=$null; FilterMs=$null; ModsMs=$null; ExitCode=$null}
        try {
            while ($sw.Elapsed.TotalSeconds -lt 35) {
                $p.Refresh()
                if ($p.HasExited) { throw 'Host exited before ready' }
                $h = $p.MainWindowHandle
                if ($h -ne [IntPtr]::Zero) {
                    if ($null -eq $row.HostMs) { $row.HostMs=$sw.ElapsedMilliseconds }
                    foreach ($spec in @(@('L4D2AutobhopVPKW','BhopMs',15),@('L4D2RowFilterManagerWindow','FilterMs',25),@('L4D2ModJoinWindow','ModsMs',15))) {
                        if ($null -ne $row[$spec[1]]) { continue }
                        $ch = [StartupProbe]::Find($h,$spec[0])
                        if ($ch -ne [IntPtr]::Zero -and [StartupProbe]::Count($ch) -ge $spec[2] -and (-not $RequireReady -or ([StartupProbe]::GetProp($ch,"L4D2MixReady") -ne [IntPtr]::Zero -and [StartupProbe]::GetProp($ch,"L4D2MixAttached") -ne [IntPtr]::Zero))) { $row[$spec[1]]=$sw.ElapsedMilliseconds }
                    }
                    if ($null -ne $row.BhopMs -and $null -ne $row.FilterMs -and $null -ne $row.ModsMs) { break }
                }
                Start-Sleep -Milliseconds 20
            }
            if ($null -eq $row.BhopMs -or $null -eq $row.FilterMs -or $null -eq $row.ModsMs) { throw 'Startup readiness timeout' }
        } finally {
            $p.Refresh()
            if (-not $p.HasExited -and $p.MainWindowHandle -ne [IntPtr]::Zero) {
                [void][StartupProbe]::PostMessage($p.MainWindowHandle,0x10,[IntPtr]::Zero,[IntPtr]::Zero)
            }
            if (-not $p.WaitForExit(10000)) { $p.Kill(); throw 'Host close timeout' }
            $row.ExitCode=$p.ExitCode
            $results += [pscustomobject]$row
            $results | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $OutputPath -Encoding UTF8
        }
        Start-Sleep -Milliseconds 300
    }
} finally { $env:LOCALAPPDATA=$oldCache; $env:L4D2_MIX_HOST_ROOT=$oldRoot; $env:L4D2_MIX_START_NS=$oldOrigin }
$results | Format-Table -AutoSize
