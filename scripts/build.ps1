<#
.SYNOPSIS
  Builds release\bluelatch.exe (command line) and release\bluelatch-gui.exe (window).

.DESCRIPTION
  Static builds without cgo (BlueLatch calls the Windows Bluetooth API through
  syscalls, and the window uses a WebView2 binding that needs no C compiler).
  The version is stamped into both programs and shown by "bluelatch version" and
  in the footer of the window. The window program is built for the Windows GUI
  subsystem, so it opens no console window.

.EXAMPLE
  .\scripts\build.ps1 -Version 0.1.2
#>
[CmdletBinding()]
param(
    [string]$Version = 'dev'
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

New-Item -ItemType Directory -Force -Path release | Out-Null
$env:CGO_ENABLED = '0'

go build -trimpath -ldflags "-s -w -X main.version=$Version" -o release\bluelatch.exe .\cmd\bluelatch
if ($LASTEXITCODE -ne 0) { throw "go build of bluelatch failed (exit code $LASTEXITCODE)" }

go build -trimpath -ldflags "-H=windowsgui -s -w -X main.version=$Version" -o release\bluelatch-gui.exe .\cmd\bluelatch-gui
if ($LASTEXITCODE -ne 0) { throw "go build of bluelatch-gui failed (exit code $LASTEXITCODE)" }

Get-Item release\bluelatch.exe, release\bluelatch-gui.exe | Select-Object Name, Length, LastWriteTime
