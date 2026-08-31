[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)]
  [ValidatePattern('^\d+\.\d+\.\d+\.\d+$')]
  [string]$Version,

  [string]$BinaryPath = 'dist\fiberpulse.exe',
  [string]$OutputPath = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$resolvedBinary = (Resolve-Path (Join-Path $repositoryRoot $BinaryPath)).Path
if ([string]::IsNullOrWhiteSpace($OutputPath)) {
  $OutputPath = Join-Path $repositoryRoot "dist\FiberPulse-$Version-windows-x64.msix"
} elseif (-not [IO.Path]::IsPathRooted($OutputPath)) {
  $OutputPath = Join-Path $repositoryRoot $OutputPath
}

$makeAppx = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\*\x64\makeappx.exe" |
  Sort-Object FullName -Descending |
  Select-Object -First 1
if (-not $makeAppx) {
  throw 'Windows SDK MakeAppx.exe is unavailable.'
}

$stage = Join-Path $env:RUNNER_TEMP 'FiberPulseMsix'
if ([string]::IsNullOrWhiteSpace($env:RUNNER_TEMP)) {
  $stage = Join-Path ([IO.Path]::GetTempPath()) 'FiberPulseMsix'
}
if (Test-Path $stage) {
  Remove-Item -LiteralPath $stage -Recurse -Force
}
New-Item -ItemType Directory -Path (Join-Path $stage 'Assets') -Force | Out-Null

$manifestTemplate = Get-Content (Join-Path $PSScriptRoot 'AppxManifest.xml') -Raw
$manifest = $manifestTemplate.Replace('__FIBERPULSE_VERSION__', $Version)
[IO.File]::WriteAllText((Join-Path $stage 'AppxManifest.xml'), $manifest, [Text.UTF8Encoding]::new($false))

Copy-Item -LiteralPath $resolvedBinary -Destination (Join-Path $stage 'fiberpulse.exe')
Copy-Item -LiteralPath (Join-Path $repositoryRoot 'LICENSE') -Destination (Join-Path $stage 'LICENSE.txt')
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'FiberPulse.ico') -Destination (Join-Path $stage 'FiberPulse.ico')
Copy-Item -Path (Join-Path $PSScriptRoot 'Assets\*.png') -Destination (Join-Path $stage 'Assets')

$outputDirectory = Split-Path -Parent $OutputPath
New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
if (Test-Path $OutputPath) {
  Remove-Item -LiteralPath $OutputPath -Force
}

& $makeAppx.FullName pack /d $stage /p $OutputPath /o
if ($LASTEXITCODE -ne 0 -or -not (Test-Path $OutputPath)) {
  throw "MakeAppx failed with exit code $LASTEXITCODE."
}

$unpackDirectory = Join-Path $env:RUNNER_TEMP 'FiberPulseMsixVerification'
if ([string]::IsNullOrWhiteSpace($env:RUNNER_TEMP)) {
  $unpackDirectory = Join-Path ([IO.Path]::GetTempPath()) 'FiberPulseMsixVerification'
}
if (Test-Path $unpackDirectory) {
  Remove-Item -LiteralPath $unpackDirectory -Recurse -Force
}
& $makeAppx.FullName unpack /p $OutputPath /d $unpackDirectory /o
if ($LASTEXITCODE -ne 0) {
  throw "MSIX verification unpack failed with exit code $LASTEXITCODE."
}

foreach ($required in @(
  'AppxManifest.xml',
  'fiberpulse.exe',
  'LICENSE.txt',
  'FiberPulse.ico',
  'Assets\StoreLogo.png',
  'Assets\Square44x44Logo.png',
  'Assets\Square150x150Logo.png'
)) {
  if (-not (Test-Path (Join-Path $unpackDirectory $required))) {
    throw "MSIX is missing required file: $required"
  }
}

[xml]$verifiedManifest = Get-Content (Join-Path $unpackDirectory 'AppxManifest.xml') -Raw
$identity = $verifiedManifest.Package.Identity
if ($identity.Name -ne 'SEOWEBAPP.FiberPulse' -or
    $identity.Publisher -ne 'CN=C7998E7B-8BEC-4624-B834-4CC18C9373BD' -or
    $identity.Version -ne $Version -or
    $identity.ProcessorArchitecture -ne 'x64') {
  throw 'The generated MSIX identity does not match the Microsoft Store assignment.'
}

$hash = (Get-FileHash -LiteralPath $OutputPath -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Output "MSIX=$OutputPath"
Write-Output "SHA256=$hash"
