<#
.SYNOPSIS
  Builds release\bluelatch.exe: the command line and the window in one program.

.DESCRIPTION
  A static build without cgo (BlueLatch calls the Windows Bluetooth API through
  syscalls, and the window uses a WebView2 binding that needs no C compiler).
  The version is stamped into the program and shown by "bluelatch version" and in
  the footer of the window. The program is a console program on purpose (ADR 0008):
  a shell waits for it and sees its exit code, and the window closes the console
  that a double click gives it.

.EXAMPLE
  .\scripts\build.ps1 -Version 0.1.3
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

Get-Item release\bluelatch.exe | Select-Object Name, Length, LastWriteTime
