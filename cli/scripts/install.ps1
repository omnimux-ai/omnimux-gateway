#Requires -Version 5.1
<#
.SYNOPSIS
  Install the omnimux CLI binary on Windows.

.DESCRIPTION
  Downloads the prebuilt Windows binary from the public GitHub Releases repo
  laozhong86/omnimux-cli and installs it under the user profile (no admin).

.EXAMPLE
  # User-facing one-liner (after omnimux.ai/install.ps1 is live):
  irm https://omnimux.ai/install.ps1 | iex

.EXAMPLE
  powershell -File install.ps1 -Version 0.2.3 -Prefix "$env:LOCALAPPDATA\OmniMux\bin"
#>

param(
  [string]$Version = "latest",
  [string]$Prefix = ""
)

$ErrorActionPreference = "Stop"

$ReleaseRepo = "laozhong86/omnimux-cli"
$AssetName = "omnimux-windows-x64.exe"
$ExeName = "omnimux.exe"

function Write-Info([string]$Message) { Write-Host $Message }
function Write-Err([string]$Message) { Write-Host "error: $Message" -ForegroundColor Red }

function Normalize-Tag([string]$V) {
  if ($V -eq "latest") { return "latest" }
  $v = $V
  if ($v.StartsWith("cli-")) { $v = $v.Substring(4) }
  if ($v.StartsWith("v")) { $v = $v.Substring(1) }
  return "cli-v$v"
}

# Windows arm64 has no prebuilt asset yet
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($arch -eq "arm64") {
  Write-Err "no prebuilt windows/arm64 binary yet; install via npm: npm i -g @omnimux/cli"
  exit 1
}

$tag = Normalize-Tag $Version
if ($tag -eq "latest") {
  $base = "https://github.com/$ReleaseRepo/releases/latest/download"
} else {
  $base = "https://github.com/$ReleaseRepo/releases/download/$tag"
}

$url = "$base/$AssetName"
$sumsUrl = "$base/SHA256SUMS"

if ([string]::IsNullOrWhiteSpace($Prefix)) {
  $destDir = Join-Path $env:LOCALAPPDATA "OmniMux\bin"
} else {
  $destDir = $Prefix
}
$dest = Join-Path $destDir $ExeName

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("omnimux-install-" + [guid]::NewGuid().ToString("n") + ".exe")
$sumsTmp = Join-Path ([System.IO.Path]::GetTempPath()) ("omnimux-sums-" + [guid]::NewGuid().ToString("n") + ".txt")

try {
  Write-Info "downloading $url"
  try {
    Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing
  } catch {
    Write-Err "download failed. Check that release asset $AssetName exists on $ReleaseRepo."
    Write-Err "hint: npm i -g @omnimux/cli"
    exit 1
  }

  # Optional checksum
  try {
    Invoke-WebRequest -Uri $sumsUrl -OutFile $sumsTmp -UseBasicParsing
    $line = Get-Content $sumsTmp | Where-Object { $_ -match [regex]::Escape($AssetName) } | Select-Object -First 1
    if ($line) {
      $expected = ($line -split "\s+")[0].Trim().ToLowerInvariant()
      $actual = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLowerInvariant()
      if ($expected -and $actual -ne $expected) {
        Write-Err "SHA256 mismatch for $AssetName"
        Write-Err "  expected: $expected"
        Write-Err "  actual:   $actual"
        exit 1
      }
      if ($expected) { Write-Info "checksum ok" }
    }
  } catch {
    Write-Info "note: SHA256SUMS not found for this release; skipping checksum verification"
  }

  if (-not (Test-Path $destDir)) {
    New-Item -ItemType Directory -Path $destDir -Force | Out-Null
  }
  Move-Item -Force -Path $tmp -Destination $dest
  $tmp = $null

  Write-Info "installed: $dest"

  # User PATH
  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if (-not $userPath) { $userPath = "" }
  $parts = $userPath -split ";" | Where-Object { $_ -ne "" }
  if ($parts -notcontains $destDir) {
    $newPath = if ($userPath.Trim().Length -eq 0) { $destDir } else { "$userPath;$destDir" }
    try {
      [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
      $env:Path = "$destDir;$env:Path"
      Write-Info "added to user PATH: $destDir"
      Write-Info "note: open a new terminal if 'omnimux' is not found in this session."
    } catch {
      Write-Info "note: could not update user PATH. Add manually: $destDir"
    }
  } else {
    # Ensure current session sees it
    if (($env:Path -split ";") -notcontains $destDir) {
      $env:Path = "$destDir;$env:Path"
    }
  }

  try {
    $ver = & $dest --version 2>$null
    if ($LASTEXITCODE -eq 0 -or $ver) {
      Write-Info "omnimux $ver ready."
      Write-Info "next: omnimux help   # default instance https://omnimux.ai"
    } else {
      Write-Info "installed binary could not run --version; check architecture and permissions."
    }
  } catch {
    Write-Info "installed binary could not run --version; check architecture and permissions."
  }
} finally {
  if ($tmp -and (Test-Path $tmp)) { Remove-Item -Force $tmp -ErrorAction SilentlyContinue }
  if (Test-Path $sumsTmp) { Remove-Item -Force $sumsTmp -ErrorAction SilentlyContinue }
}
