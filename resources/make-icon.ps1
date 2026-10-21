# Generates resources/app.ico from vector shapes. Requires Windows PowerShell
# with System.Drawing. Run from the repository root:
#   powershell -ExecutionPolicy Bypass -File resources/make-icon.ps1
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$outDir = Join-Path $PSScriptRoot 'build'
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

function New-RoundedRectPath([float]$x, [float]$y, [float]$w, [float]$h, [float]$r) {
    $p = New-Object System.Drawing.Drawing2D.GraphicsPath
    $d = $r * 2
    $p.AddArc($x, $y, $d, $d, 180, 90)
    $p.AddArc($x + $w - $d, $y, $d, $d, 270, 90)
    $p.AddArc($x + $w - $d, $y + $h - $d, $d, $d, 0, 90)
    $p.AddArc($x, $y + $h - $d, $d, $d, 90, 90)
    $p.CloseFigure()
    return $p
}

$master = New-Object System.Drawing.Bitmap 1024, 1024, ([System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
$g = [System.Drawing.Graphics]::FromImage($master)
$g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
$g.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
$g.Clear([System.Drawing.Color]::Transparent)

$rect = New-Object System.Drawing.RectangleF 64, 64, 896, 896
$bg = New-RoundedRectPath 64 64 896 896 216

$grad = New-Object System.Drawing.Drawing2D.LinearGradientBrush $rect, ([System.Drawing.Color]::FromArgb(255, 91, 141, 255)), ([System.Drawing.Color]::FromArgb(255, 29, 67, 181)), 55
$blend = New-Object System.Drawing.Drawing2D.ColorBlend 3
$blend.Colors = @(
    [System.Drawing.Color]::FromArgb(255, 104, 154, 255),
    [System.Drawing.Color]::FromArgb(255, 47, 99, 232),
    [System.Drawing.Color]::FromArgb(255, 26, 61, 168)
)
$blend.Positions = @(0.0, 0.55, 1.0)
$grad.InterpolationColors = $blend
$g.FillPath($grad, $bg)

# Subtle inner rim keeps the tile crisp on light and dark backgrounds.
$rim = New-RoundedRectPath 70 70 884 884 210
$rimPen = New-Object System.Drawing.Pen ([System.Drawing.Color]::FromArgb(38, 255, 255, 255)), 6
$g.DrawPath($rimPen, $rim)

$tri = @(
    (New-Object System.Drawing.PointF 424, 300),
    (New-Object System.Drawing.PointF 424, 724),
    (New-Object System.Drawing.PointF 792, 512)
)
$white = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::White)
$triangle = New-Object System.Drawing.Drawing2D.GraphicsPath
$triangle.AddPolygon($tri)
$g.FillPath($white, $triangle)
# Stroke with round joins to soften the three corners.
$triPen = New-Object System.Drawing.Pen ([System.Drawing.Color]::White), 42
$triPen.LineJoin = [System.Drawing.Drawing2D.LineJoin]::Round
$g.DrawPath($triPen, $triangle)

$master.Save((Join-Path $outDir 'icon-1024.png'), [System.Drawing.Imaging.ImageFormat]::Png)

$sizes = @(256, 128, 64, 48, 32, 24, 16)
foreach ($s in $sizes) {
    $bmp = New-Object System.Drawing.Bitmap $s, $s, ([System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $sg = [System.Drawing.Graphics]::FromImage($bmp)
    $sg.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $sg.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $sg.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $sg.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
    $sg.Clear([System.Drawing.Color]::Transparent)
    $sg.DrawImage($master, (New-Object System.Drawing.Rectangle 0, 0, $s, $s), 0, 0, 1024, 1024, [System.Drawing.GraphicsUnit]::Pixel)
    $sg.Dispose()
    $bmp.Save((Join-Path $outDir ("icon-$s.png")), [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
}

# ICO container with PNG-compressed frames (supported by Windows Vista+).
$icoPath = Join-Path $PSScriptRoot 'app.ico'
$fs = [System.IO.File]::Create($icoPath)
$bw = New-Object System.IO.BinaryWriter($fs)
$bw.Write([UInt16]0)          # reserved
$bw.Write([UInt16]1)          # type: icon
$bw.Write([UInt16]$sizes.Count)
$offset = 6 + (16 * $sizes.Count)
$frames = @()
foreach ($s in $sizes) {
    $bytes = [System.IO.File]::ReadAllBytes((Join-Path $outDir ("icon-$s.png")))
    $frames += ,$bytes
    $dim = if ($s -ge 256) { 0 } else { $s }
    $bw.Write([Byte]$dim)     # width (0 = 256)
    $bw.Write([Byte]$dim)     # height
    $bw.Write([Byte]0)        # palette
    $bw.Write([Byte]0)        # reserved
    $bw.Write([UInt16]1)      # planes
    $bw.Write([UInt16]32)     # bits per pixel
    $bw.Write([UInt32]$bytes.Length)
    $bw.Write([UInt32]$offset)
    $offset += $bytes.Length
}
foreach ($bytes in $frames) { $bw.Write($bytes) }
$bw.Close()

$g.Dispose()
$master.Dispose()
Write-Host "wrote $icoPath"
