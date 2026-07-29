[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$From,
    [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$To,
    [switch]$ReplaceExisting
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\..\scripts\build-publication.ps1")

Move-BuildPublicationFileDurably `
    -From $From `
    -To $To `
    -ReplaceExisting:$ReplaceExisting
