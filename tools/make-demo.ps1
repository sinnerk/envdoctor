# Renders docs/demo.gif from the real output of envdoctor.
#
# The demo is not a mockup: it runs the built binary, captures exactly what it
# prints, and animates that text into a terminal-style GIF. Re-run this after a
# release that changes the report layout.
#
# Requires ffmpeg on PATH. Usage:
#   .\tools\make-demo.ps1

[CmdletBinding()]
param(
    [string]$Exe,
    [string]$Out,
    [int]$Fps = 10,
    [int]$Columns = 30,
    [string]$Command = 'envdoctor',
    # Keep the intermediate PNGs for troubleshooting the encode.
    [switch]$KeepFrames
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

# Resolve defaults here: $PSScriptRoot is not reliably populated while
# PowerShell evaluates parameter defaults.
if (-not $Exe) { $Exe = Join-Path $PSScriptRoot '..\envdoctor.exe' }
if (-not $Out) { $Out = Join-Path $PSScriptRoot '..\docs\demo.gif' }

if (-not (Test-Path -LiteralPath $Exe)) {
    throw "Build the binary first: go build -o envdoctor.exe ./cmd/envdoctor"
}
if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) {
    throw 'ffmpeg is required to encode the GIF.'
}

# ---- capture the real report -------------------------------------------------
# Native output is UTF-8 (the report contains a middle dot); without this the
# console code page mangles it before it reaches the renderer.
$previousEncoding = [Console]::OutputEncoding
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
try {
    $output = & $Exe --no-color *>&1 | ForEach-Object { $_.ToString() }
} finally {
    [Console]::OutputEncoding = $previousEncoding
}
$lines = @($output)

# ---- layout ------------------------------------------------------------------
$font = New-Object System.Drawing.Font('Consolas', 15, [System.Drawing.FontStyle]::Regular, [System.Drawing.GraphicsUnit]::Pixel)
$bold = New-Object System.Drawing.Font('Consolas', 15, [System.Drawing.FontStyle]::Bold, [System.Drawing.GraphicsUnit]::Pixel)

$probe = New-Object System.Drawing.Bitmap 8, 8
$pg = [System.Drawing.Graphics]::FromImage($probe)
$charWidth = $pg.MeasureString('MMMMMMMMMM', $font, 10000, [System.Drawing.StringFormat]::GenericTypographic).Width / 10
$pg.Dispose()
$probe.Dispose()

$pad = 22
$lineHeight = 21
$promptRows = 2
$maxLen = ($lines | Measure-Object -Maximum Length).Maximum
$width = [int][Math]::Max(880, [Math]::Ceiling(($maxLen + 3) * $charWidth) + $pad * 2)
$height = $pad * 2 + $promptRows * $lineHeight + $Columns * $lineHeight

$colors = @{
    bg     = [System.Drawing.Color]::FromArgb(255, 13, 17, 23)
    text   = [System.Drawing.Color]::FromArgb(255, 201, 209, 217)
    dim    = [System.Drawing.Color]::FromArgb(255, 139, 148, 158)
    red    = [System.Drawing.Color]::FromArgb(255, 248, 81, 73)
    cyan   = [System.Drawing.Color]::FromArgb(255, 121, 192, 255)
    green  = [System.Drawing.Color]::FromArgb(255, 126, 231, 135)
    prompt = [System.Drawing.Color]::FromArgb(255, 126, 231, 135)
}

function Get-LineStyle {
    param([string]$Line)

    if ($Line -match '^\s+!') { return @{ Color = $colors.red } }
    if ($Line -match '^\s+-') { return @{ Color = $colors.cyan } }
    if ($Line -match '^\s+fix:') { return @{ Color = $colors.dim } }
    if ($Line -match 'grade') {
        if ($Line -match 'grade [AB]') { return @{ Color = $colors.green } }
        return @{ Color = $colors.red }
    }
    if ($Line -match '^\s+\d+ of \d+|^all .* clean') { return @{ Color = $colors.dim } }
    return @{ Color = $colors.text }
}

$framesDir = Join-Path $env:TEMP ('envdoctor-frames-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $framesDir | Out-Null

$typeFrames = 10
$revealFrames = [int]($lines.Count / 0.72)
$bottomHold = 12
$returnFrames = 12
$topHold = 30
$totalFrames = $typeFrames + $revealFrames + $bottomHold + $returnFrames + $topHold
$cursorOn = $true

function New-Frame {
    param([int]$Index)

    $bmp = New-Object System.Drawing.Bitmap($width, $height)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.Clear($colors.bg)
    $g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias

    # prompt line
    $y = $pad
    $promptText = 'PS C:\work> '
    $promptBrush = New-Object System.Drawing.SolidBrush($colors.prompt)
    $g.DrawString($promptText, $bold, $promptBrush, $pad, $y, [System.Drawing.StringFormat]::GenericTypographic)
    $promptWidth = $g.MeasureString($promptText, $bold, 10000, [System.Drawing.StringFormat]::GenericTypographic).Width

    $typed = $Command
    if ($Index -lt $typeFrames) {
        $n = [int][Math]::Ceiling($Command.Length * ($Index + 1) / $typeFrames)
        $typed = $Command.Substring(0, $n)
    }
    $textBrush = New-Object System.Drawing.SolidBrush($colors.text)
    # +1 character: GenericTypographic trims the trailing space of the prompt.
    $commandX = $pad + $promptWidth + $charWidth
    $g.DrawString($typed, $font, $textBrush, $commandX, $y, [System.Drawing.StringFormat]::GenericTypographic)

    # a cursor block while the command is being typed
    if ($Index -lt $typeFrames -and $cursorOn) {
        $cursorX = $commandX + $g.MeasureString($typed, $font, 10000, [System.Drawing.StringFormat]::GenericTypographic).Width
        $cursorBrush = New-Object System.Drawing.SolidBrush($colors.dim)
        $g.FillRectangle($cursorBrush, [int]$cursorX + 1, $y + 2, [int]$charWidth, $lineHeight - 6)
        $cursorBrush.Dispose()
    }

    # Reveal, then hold at the bottom, scroll back to the score, hold again.
    $revealStart = $typeFrames
    $bottomStart = $revealStart + $revealFrames
    $returnStart = $bottomStart + $bottomHold

    $revealed = 0
    $offset = 0
    $maxOffset = [Math]::Max(0, $lines.Count - $Columns)

    if ($Index -ge $revealStart) {
        $revealed = [int][Math]::Floor(($Index - $revealStart) * 0.72)
        if ($revealed -gt $lines.Count) { $revealed = $lines.Count }
        $offset = [Math]::Max(0, $revealed - $Columns)
    }
    if ($Index -ge $bottomStart) {
        $revealed = $lines.Count
        $offset = $maxOffset
    }
    if ($Index -ge $returnStart) {
        $revealed = $lines.Count
        $progress = ($Index - $returnStart) / [double]$returnFrames
        if ($progress -ge 1) {
            $offset = 0
        } else {
            $offset = [int][Math]::Round($maxOffset * (1 - $progress))
        }
    }

    $startY = $pad + $promptRows * $lineHeight
    for ($i = 0; $i -lt $Columns; $i++) {
        $lineIndex = $offset + $i
        if ($lineIndex -ge $revealed) { break }
        $line = $lines[$lineIndex]
        if ($null -eq $line) { $line = '' }
        $style = Get-LineStyle -Line $line
        $brush = New-Object System.Drawing.SolidBrush($style.Color)
        $g.DrawString($line, $font, $brush, $pad, $startY + $i * $lineHeight, [System.Drawing.StringFormat]::GenericTypographic)
        $brush.Dispose()
    }

    $path = Join-Path $framesDir ('f_{0:d4}.png' -f $Index)
    $bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    $g.Dispose()
    $bmp.Dispose()

    $promptBrush.Dispose()
    $textBrush.Dispose()
}

for ($f = 0; $f -lt $totalFrames; $f++) {
    $cursorOn = ($f % 6) -lt 3
    New-Frame -Index $f
    if ($f % 20 -eq 0) { Write-Host ("frame {0}/{1}" -f $f, $totalFrames) }
}

# ---- encode ------------------------------------------------------------------
$outDir = Split-Path -Parent $Out
if (-not (Test-Path -LiteralPath $outDir)) { New-Item -ItemType Directory -Path $outDir | Out-Null }
if (Test-Path -LiteralPath $Out) { Remove-Item -LiteralPath $Out -Force }

$filter = 'split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=3'
& ffmpeg -y -loglevel error -framerate $Fps -i (Join-Path $framesDir 'f_%04d.png') -vf $filter -loop 0 $Out
if ($LASTEXITCODE -ne 0) { throw "ffmpeg failed with exit code $LASTEXITCODE" }

if (-not $KeepFrames) { Remove-Item -LiteralPath $framesDir -Recurse -Force }

$size = (Get-Item -LiteralPath $Out).Length
Write-Host ("wrote {0} ({1:N0} KB, {2} frames at {3} fps)" -f $Out, ($size / 1KB), $totalFrames, $Fps)
if ($KeepFrames) { Write-Host "frames kept in $framesDir" }
