<#
.SYNOPSIS
  Renders the PNG frames of the icon from assets\logo\bluelatch.svg into assets\logo\png.

.DESCRIPTION
  Writes bluelatch-N.png for N = 16, 20, 24, 32, 40, 48, 64, 128, 256 (the frames of the icon
  embedded in the programs; see scripts\resources.ps1) and 512 (for the README and a repository
  picture). Each frame is the SVG rendered at exactly N x N pixels with a transparent background.

  The SVG is drawn by headless Chrome, because it uses a filter and clip paths. The page shows it at
  the top left corner of a window that is large enough, and the screenshot is cropped to N x N.
  Needs Google Chrome (the path can be given with -Chrome).

.EXAMPLE
  .\scripts\logo-frames.ps1
#>
[CmdletBinding()]
param(
    [string]$Chrome = 'C:\Program Files\Google\Chrome\Application\chrome.exe'
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
Add-Type -AssemblyName System.Drawing

if (-not (Test-Path $Chrome)) { throw "Chrome not found at $Chrome; give the path with -Chrome" }
$svg = (Resolve-Path 'assets\logo\bluelatch.svg').Path
$outDir = Join-Path (Get-Location) 'assets\logo\png'
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$work = Join-Path ([IO.Path]::GetTempPath()) 'bluelatch-logo-frames'
New-Item -ItemType Directory -Force -Path $work | Out-Null

foreach ($n in 16, 20, 24, 32, 40, 48, 64, 128, 256, 512) {
    $page = Join-Path $work "frame-$n.html"
    $svgUrl = 'file:///' + $svg.Replace('\', '/')
    [IO.File]::WriteAllText($page, "<!doctype html><meta charset=utf-8><style>html,body{margin:0;background:transparent}img{display:block;width:${n}px;height:${n}px}</style><img src=`"$svgUrl`">", (New-Object Text.UTF8Encoding($false)))
    $raw = Join-Path $work "frame-$n-raw.png"
    Remove-Item $raw -ErrorAction SilentlyContinue
    $size = [Math]::Max(600, $n + 40)
    $chromeArgs = @('--headless=new', '--disable-gpu', '--hide-scrollbars', '--default-background-color=00000000',
        '--force-device-scale-factor=1', "--window-size=$size,$size", "--screenshot=$raw",
        "--user-data-dir=$(Join-Path $work 'profile')", ('file:///' + $page.Replace('\', '/')))
    Start-Process -FilePath $Chrome -ArgumentList $chromeArgs -Wait
    if (-not (Test-Path $raw)) { throw "Chrome made no screenshot for $n px" }

    $source = [System.Drawing.Bitmap]::FromFile($raw)
    try {
        $frame = New-Object System.Drawing.Bitmap -ArgumentList $n, $n, ([System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
        $g = [System.Drawing.Graphics]::FromImage($frame)
        $g.DrawImage($source, (New-Object System.Drawing.Rectangle(0, 0, $n, $n)), (New-Object System.Drawing.Rectangle(0, 0, $n, $n)), [System.Drawing.GraphicsUnit]::Pixel)
        $g.Dispose()
        $target = Join-Path $outDir "bluelatch-$n.png"
        $frame.Save($target, [System.Drawing.Imaging.ImageFormat]::Png)
        # The corner of a rounded tile must be transparent, and the middle opaque.
        if ($frame.GetPixel(0, 0).A -ne 0 -or $frame.GetPixel([int]($n / 2), [int]($n / 2)).A -ne 255) { throw "frame $n px has the wrong transparency" }
        $frame.Dispose()
    }
    finally { $source.Dispose() }
    '{0,4} px  {1,7} bytes' -f $n, (Get-Item $target).Length
}
