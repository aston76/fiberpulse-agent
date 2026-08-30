$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$Repository = 'aston76/fiberpulse-agent'
$ReleaseApi = "https://api.github.com/repos/$Repository/releases/latest"
$Headers = @{
    Accept = 'application/vnd.github+json'
    'User-Agent' = 'FiberPulse-Installer'
}

try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $Release = Invoke-RestMethod -Uri $ReleaseApi -Headers $Headers
}
catch {
    throw 'FiberPulse installation stopped: no signed public release is available yet.'
}

$Asset = $Release.assets |
    Where-Object { $_.name -match '^FiberPulse-.*-windows-x64-setup\.exe$' } |
    Select-Object -First 1

if ($null -eq $Asset) {
    throw 'FiberPulse installation stopped: the latest release does not contain the expected Windows installer.'
}

$ExpectedPrefix = "https://github.com/$Repository/releases/download/"
if (-not $Asset.browser_download_url.StartsWith($ExpectedPrefix, [StringComparison]::Ordinal)) {
    throw 'FiberPulse installation stopped: the installer download address is not trusted.'
}

$InstallTempDir = Join-Path ([IO.Path]::GetTempPath()) ("fiberpulse-install-" + [Guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Path $InstallTempDir
$InstallerPath = Join-Path $InstallTempDir $Asset.name

try {
    Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $InstallerPath -UseBasicParsing

    $Signature = Get-AuthenticodeSignature -FilePath $InstallerPath
    if ($Signature.Status -ne [System.Management.Automation.SignatureStatus]::Valid) {
        throw "FiberPulse installation stopped: the installer signature is $($Signature.Status)."
    }

    $Installer = Start-Process -FilePath $InstallerPath -PassThru -Wait
    if ($Installer.ExitCode -ne 0) {
        throw "FiberPulse installer exited with code $($Installer.ExitCode)."
    }

    Write-Host 'FiberPulse was installed successfully.' -ForegroundColor Green
}
finally {
    Remove-Item -LiteralPath $InstallTempDir -Recurse -Force -ErrorAction SilentlyContinue
}
