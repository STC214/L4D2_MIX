param([Parameter(Mandatory=$true)][string]$Exe,[Parameter(Mandatory=$true)][string]$FixtureRoot,[Parameter(Mandatory=$true)][string]$OutputDir)
$ErrorActionPreference='Stop'
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
    [DllImport("user32.dll")] public static extern IntPtr SendMessage(IntPtr h, uint m, IntPtr w, IntPtr l);
    [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr h, int id);
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
$Exe=(Resolve-Path -LiteralPath $Exe).Path
New-Item -ItemType Directory -Force -Path $FixtureRoot,$OutputDir | Out-Null
$FixtureRoot=(Resolve-Path -LiteralPath $FixtureRoot).Path
$OutputDir=(Resolve-Path -LiteralPath $OutputDir).Path
$oldCache=$env:LOCALAPPDATA; $oldRoot=$env:L4D2_MIX_HOST_ROOT
$results=@()
try {
 foreach ($delay in @(0,80,150,250,450,1200)) {
  $env:LOCALAPPDATA=Join-Path $FixtureRoot "close-$delay/cache"
  $env:L4D2_MIX_HOST_ROOT=Join-Path $FixtureRoot "close-$delay/host"
  New-Item -ItemType Directory -Force -Path $env:LOCALAPPDATA,$env:L4D2_MIX_HOST_ROOT | Out-Null
  $p=Start-Process -FilePath $Exe -WindowStyle Hidden -PassThru
  $watch=[Diagnostics.Stopwatch]::StartNew()
  $h=[IntPtr]::Zero
  while ($watch.Elapsed.TotalSeconds -lt 15) { $p.Refresh(); $h=$p.MainWindowHandle; if ($h -ne [IntPtr]::Zero) {break}; if($p.HasExited){throw 'early host exit'}; Start-Sleep -Milliseconds 10 }
  if($h -eq [IntPtr]::Zero){throw 'host window timeout'}
  Start-Sleep -Milliseconds ([Math]::Max(1,$delay))
  [void][StartupProbe]::PostMessage($h,0x10,[IntPtr]::Zero,[IntPtr]::Zero)
  if(-not $p.WaitForExit(10000)){ $p.Kill(); throw "Close timeout: $delay" }
  Start-Sleep -Milliseconds 200
  $orphans=@(Get-Process L4D2* -ErrorAction SilentlyContinue | Where-Object { $_.Path -and $_.Path.StartsWith($env:LOCALAPPDATA,[StringComparison]::OrdinalIgnoreCase) })
  if($orphans.Count -gt 0){throw "Orphan children after close: $delay"}
  $results += [pscustomobject]@{CloseDelayMs=$delay;ExitCode=$p.ExitCode;Orphans=$orphans.Count}
 }
 $env:LOCALAPPDATA=Join-Path $FixtureRoot 'ui/cache'; $env:L4D2_MIX_HOST_ROOT=Join-Path $FixtureRoot 'ui/host'
 New-Item -ItemType Directory -Force -Path $env:LOCALAPPDATA,$env:L4D2_MIX_HOST_ROOT | Out-Null
 $p=Start-Process -FilePath $Exe -WindowStyle Hidden -PassThru
 try {
  $watch=[Diagnostics.Stopwatch]::StartNew(); $ready=$false
  while($watch.Elapsed.TotalSeconds -lt 25) {
   $p.Refresh(); $h=$p.MainWindowHandle
   if($h -ne [IntPtr]::Zero) {
    $ready=$true
    foreach($cls in @('L4D2AutobhopVPKW','L4D2RowFilterManagerWindow','L4D2ModJoinWindow')) {
     $ch=[StartupProbe]::Find($h,$cls)
     if($ch -eq [IntPtr]::Zero -or [StartupProbe]::GetProp($ch,'L4D2MixReady') -eq [IntPtr]::Zero){$ready=$false}
    }
    if($ready){break}
   }
   Start-Sleep -Milliseconds 25
  }
  if(-not $ready){throw 'all pages ready timeout'}
  & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'verify-ui-switch.ps1') -OutputPath (Join-Path $OutputDir 'switch-results.json') -ScreenshotDir (Join-Path $OutputDir 'screenshots')
  if($LASTEXITCODE -ne 0){throw 'switch script failed'}
  [void][StartupProbe]::SendMessage($h,0x112,[IntPtr]0xF020,[IntPtr]::Zero)
  Start-Sleep -Milliseconds 100
  if([StartupProbe]::IsWindowVisible($h)){throw 'tray minimize failed'}
  [void][StartupProbe]::SendMessage($h,0x8003,[IntPtr]::Zero,[IntPtr]0x202)
  Start-Sleep -Milliseconds 200
  if(-not [StartupProbe]::IsWindowVisible($h)){throw 'tray restore failed'}
  $mods=[StartupProbe]::Find($h,'L4D2ModJoinWindow')
  if(-not [StartupProbe]::IsWindowVisible($mods)){throw 'current page missing after restore'}
  # Simulate an unexpected child exit, ensuring host remains usable and closes normally.
  $filter=@(Get-Process L4D2RowFilterManager -ErrorAction SilentlyContinue | Where-Object { $_.Path.StartsWith($env:LOCALAPPDATA,[StringComparison]::OrdinalIgnoreCase) })
  if($filter.Count -ne 1){throw 'fixture filter process mismatch'}
  $filter[0].Kill(); [void]$filter[0].WaitForExit(5000)
  Start-Sleep -Milliseconds 200
  $button=[StartupProbe]::GetDlgItem($h,102)
  [void][StartupProbe]::SendMessage($button,0xF5,[IntPtr]::Zero,[IntPtr]::Zero)
  $p.Refresh(); if(-not $p.Responding){throw 'host unresponsive after child exit'}
  $results += [pscustomobject]@{Switches=30;TrayRestore=$true;ChildExitHandled=$true}
 } finally {
  $p.Refresh()
  if(-not $p.HasExited) { [void][StartupProbe]::PostMessage($h,0x10,[IntPtr]::Zero,[IntPtr]::Zero); if(-not $p.WaitForExit(10000)){ $p.Kill(); throw 'UI host close timeout'} }
 }
 $results | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $OutputDir 'lifecycle-results.json') -Encoding UTF8
 Write-Host 'PASS: six startup-close timings, zero orphan children, 30 switches, tray restore, unexpected child exit.'
} finally { $env:LOCALAPPDATA=$oldCache; $env:L4D2_MIX_HOST_ROOT=$oldRoot }
