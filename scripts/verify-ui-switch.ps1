param(
    [Parameter(Mandatory = $true)]
    [string]$OutputPath,
    [string]$ScreenshotDir = ""
)

$ErrorActionPreference = "Stop"
trap {
    [pscustomobject]@{
        Error = $_.Exception.Message
        Detail = $_ | Out-String
    } | ConvertTo-Json | Set-Content -LiteralPath $OutputPath -Encoding UTF8
    exit 1
}

Add-Type @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;

public static class MixVerify {
    public delegate bool EnumProc(IntPtr hwnd, IntPtr lParam);

    [DllImport("user32.dll")]
    public static extern bool EnumChildWindows(IntPtr parent, EnumProc callback, IntPtr lParam);

    [DllImport("user32.dll")]
    public static extern int GetClassName(IntPtr hwnd, StringBuilder text, int count);

    [DllImport("user32.dll")]
    public static extern int GetWindowText(IntPtr hwnd, StringBuilder text, int count);

    [DllImport("user32.dll")]
    public static extern bool IsWindowVisible(IntPtr hwnd);

    [DllImport("user32.dll")]
    public static extern bool IsWindowEnabled(IntPtr hwnd);

    [DllImport("user32.dll")]
    public static extern IntPtr SendMessage(IntPtr hwnd, uint message, IntPtr wParam, IntPtr lParam);

    [DllImport("user32.dll")]
    public static extern IntPtr GetDlgItem(IntPtr parent, int id);

    [StructLayout(LayoutKind.Sequential)]
    public struct Rect {
        public int Left;
        public int Top;
        public int Right;
        public int Bottom;
    }

    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr hwnd, out Rect rect);
}
'@

if ($ScreenshotDir) {
    Add-Type -AssemblyName System.Drawing
    New-Item -ItemType Directory -Force -Path $ScreenshotDir | Out-Null
}

function Save-WindowScreenshot([IntPtr]$Window, [string]$Path) {
    if (-not $ScreenshotDir) {
        return
    }
    $rect = New-Object MixVerify+Rect
    [void][MixVerify]::GetWindowRect($Window, [ref]$rect)
    $bitmap = New-Object Drawing.Bitmap ($rect.Right - $rect.Left), ($rect.Bottom - $rect.Top)
    $graphics = [Drawing.Graphics]::FromImage($bitmap)
    $graphics.CopyFromScreen($rect.Left, $rect.Top, 0, 0, $bitmap.Size)
    $graphics.Dispose()
    $bitmap.Save($Path, [Drawing.Imaging.ImageFormat]::Png)
    $bitmap.Dispose()
}

function Get-Children([IntPtr]$Root) {
    $script:found = @()
    $callback = [MixVerify+EnumProc]{
        param($hwnd, $lParam)
        $class = New-Object Text.StringBuilder 128
        $text = New-Object Text.StringBuilder 256
        [void][MixVerify]::GetClassName($hwnd, $class, $class.Capacity)
        [void][MixVerify]::GetWindowText($hwnd, $text, $text.Capacity)
        $script:found += [pscustomobject]@{
            Handle = $hwnd
            Class = $class.ToString()
            Text = $text.ToString()
            Visible = [MixVerify]::IsWindowVisible($hwnd)
            Enabled = [MixVerify]::IsWindowEnabled($hwnd)
        }
        return $true
    }
    [void][MixVerify]::EnumChildWindows($Root, $callback, [IntPtr]::Zero)
    return $script:found
}

$process = Get-Process L4D2_MIX | Select-Object -First 1
$tabs = @(
    @{ Name = "bhop"; ID = 101 },
    @{ Name = "filter"; ID = 102 },
    @{ Name = "mods"; ID = 103 }
)
$results = @()
foreach ($iteration in 1..30) {
    $tab = $tabs[($iteration - 1) % $tabs.Count]
    $selected = $tab.Name
    $buttonID = $tab.ID
    $button = [MixVerify]::GetDlgItem([IntPtr]$process.MainWindowHandle, $buttonID)
    [void][MixVerify]::SendMessage($button, 0x00F5, [IntPtr]::Zero, [IntPtr]::Zero)
    Start-Sleep -Milliseconds 80

    $pages = Get-Children ([IntPtr]$process.MainWindowHandle) | Where-Object {
        $_.Class -in @("L4D2AutobhopVPKW", "L4D2RowFilterManagerWindow", "L4D2ModJoinWindow")
    }
    $bhopVisible = ($pages | Where-Object Class -eq "L4D2AutobhopVPKW").Visible
    $filterVisible = ($pages | Where-Object Class -eq "L4D2RowFilterManagerWindow").Visible
    $modsVisible = ($pages | Where-Object Class -eq "L4D2ModJoinWindow").Visible
    $bhopEnabled = ($pages | Where-Object Class -eq "L4D2AutobhopVPKW").Enabled
    $filterEnabled = ($pages | Where-Object Class -eq "L4D2RowFilterManagerWindow").Enabled
    $modsEnabled = ($pages | Where-Object Class -eq "L4D2ModJoinWindow").Enabled
    $expected = switch ($selected) {
        "bhop" {
            $bhopVisible -and $bhopEnabled -and
            -not $filterVisible -and -not $filterEnabled -and
            -not $modsVisible -and -not $modsEnabled
        }
        "filter" {
            $filterVisible -and $filterEnabled -and
            -not $bhopVisible -and -not $bhopEnabled -and
            -not $modsVisible -and -not $modsEnabled
        }
        "mods" {
            $modsVisible -and $modsEnabled -and
            -not $bhopVisible -and -not $bhopEnabled -and
            -not $filterVisible -and -not $filterEnabled
        }
    }
    $results += [pscustomobject]@{
        Iteration = $iteration
        Selected = $selected
        BhopVisible = $bhopVisible
        FilterVisible = $filterVisible
        ModsVisible = $modsVisible
        BhopEnabled = $bhopEnabled
        FilterEnabled = $filterEnabled
        ModsEnabled = $modsEnabled
        Expected = $expected
        Responding = (Get-Process -Id $process.Id).Responding
    }
    if ($ScreenshotDir -and $iteration -le 3) {
        Save-WindowScreenshot ([IntPtr]$process.MainWindowHandle) `
            (Join-Path $ScreenshotDir ("page-" + $selected + ".png"))
    }
}

$results | ConvertTo-Json | Set-Content -LiteralPath $OutputPath -Encoding UTF8
