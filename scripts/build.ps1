<#
.SYNOPSIS
  Builds release\bluelatch.exe.

.DESCRIPTION
  A static build without cgo (BlueLatch calls the Windows Bluetooth API through
  syscalls, so it needs no C compiler). The version is stamped into the binary
  and shown by "bluelatch version".

.EXAMPLE
  .\scripts\build.ps1 -Version 0.1.0
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
if ($LASTEXITCODE -ne 0) { throw "go build failed (exit code $LASTEXITCODE)" }

Get-Item release\bluelatch.exe | Select-Object FullName, Length, LastWriteTime
