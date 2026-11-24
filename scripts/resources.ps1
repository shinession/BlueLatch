<#
.SYNOPSIS
  Regenerates the .syso files that give the program its icon and its version
  information (the Details tab of the file properties).

.DESCRIPTION
  Writes cmd\bluelatch\rsrc_windows_<arch>.syso for amd64 and arm64, from the icon
  frames in assets\logo\png (bluelatch-N.png, N up to 256). The Go toolchain links a
  .syso file that sits in the directory of a main package, so building needs no tool:
  run this script only when the icon or the version changes, and commit the files.
  tools\mkrsrc does the work, with the standard library only.

.EXAMPLE
  .\scripts\resources.ps1 -Version 0.1.3
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Version
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

foreach ($arch in 'amd64', 'arm64') {
    $out = "cmd\bluelatch\rsrc_windows_$arch.syso"
    go run .\tools\mkrsrc -arch $arch -out $out -icons assets\logo\png -version $Version `
        -product-name BlueLatch -description BlueLatch -original-filename bluelatch.exe -internal-name bluelatch
    if ($LASTEXITCODE -ne 0) { throw "mkrsrc failed for $out (exit code $LASTEXITCODE)" }
}

Get-ChildItem cmd -Recurse -Filter *.syso | Select-Object @{ n = 'File'; e = { $_.FullName.Substring((Get-Location).Path.Length + 1) } }, Length
