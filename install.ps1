# Install Jin on Windows from the latest release.
#
#     irm https://raw.githubusercontent.com/jojin1709/jin/main/install.ps1 | iex
#
# It works out the processor, downloads the matching archive from the newest
# release, checks it against the published checksum, and puts jin in
# %LOCALAPPDATA%\jin, which it then adds to your user PATH.

$ErrorActionPreference = "Stop"

$Repo = "jojin1709/jin"
$InstallDir = Join-Path $env:LOCALAPPDATA "jin"

function Fail([string]$Message) {
    [Console]::Error.WriteLine("install.ps1: $Message")
    exit 1
}

# ------------------------------------------------------------- the machine

if ($PSVersionTable.PSVersion.Major -lt 6) {
    $OnWindows = $true
} else {
    $OnWindows = $IsWindows
}

if (-not $OnWindows) {
    Fail "this script installs Jin on Windows. On macOS or Linux, use install.sh."
}

switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $Arch = "amd64" }
    "ARM64" { $Arch = "arm64" }
    default {
        Fail "a $($env:PROCESSOR_ARCHITECTURE) processor is not supported. The releases are for amd64 and arm64."
    }
}

# -------------------------------------------------------------- the release

$Latest = "https://api.github.com/repos/$Repo/releases/latest"

try {
    $Release = Invoke-RestMethod -Uri $Latest -Headers @{ "User-Agent" = "jin-install" }
} catch {
    $Status = 0
    if ($_.Exception.PSObject.Properties.Name -contains "Response" -and $_.Exception.Response) {
        $Status = [int]$_.Exception.Response.StatusCode
    }
    if ($Status -eq 404) {
        Fail "there is no release to install yet. The releases are at https://github.com/$Repo/releases"
    }
    Fail "could not reach GitHub to find the latest release. $($_.Exception.Message)"
}

$Tag = $Release.tag_name
if (-not $Tag) {
    Fail "there is no release to install yet. The releases are at https://github.com/$Repo/releases"
}

$Name = "jin_${Tag}_windows_${Arch}"
$Base = "https://github.com/$Repo/releases/download/$Tag"

# ---------------------------------------------------------------- download

$Temp = Join-Path ([System.IO.Path]::GetTempPath()) ("jin-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $Temp -Force | Out-Null

try {
    Write-Host "downloading jin $Tag for windows/$Arch..."

    $Archive = Join-Path $Temp "$Name.zip"
    $Sums = Join-Path $Temp "SHA256SUMS.txt"

    try {
        Invoke-WebRequest -Uri "$Base/$Name.zip" -OutFile $Archive -UseBasicParsing
        Invoke-WebRequest -Uri "$Base/SHA256SUMS.txt" -OutFile $Sums -UseBasicParsing
    } catch {
        Fail "could not download the release from $Base. $($_.Exception.Message)"
    }

    # ------------------------------------------------------------- checksum

    $Expected = $null
    foreach ($Line in Get-Content $Sums) {
        $Parts = $Line -split '\s+', 2
        if ($Parts.Count -eq 2) {
            $File = $Parts[1].TrimStart('*')
            if ($File -eq "$Name.zip") {
                $Expected = $Parts[0].ToLower()
                break
            }
        }
    }

    if (-not $Expected) {
        Fail "the checksum file does not mention $Name.zip, so the download cannot be checked."
    }

    $Actual = (Get-FileHash -Path $Archive -Algorithm SHA256).Hash.ToLower()

    if ($Expected -ne $Actual) {
        Fail "the checksum does not match. The download may be damaged, so nothing was installed."
    }

    # -------------------------------------------------------------- install

    Expand-Archive -Path $Archive -DestinationPath $Temp -Force

    $Binary = Get-ChildItem -Path $Temp -Recurse -Filter "jin.exe" | Select-Object -First 1
    if (-not $Binary) {
        Fail "the archive did not contain a jin.exe."
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path $Binary.FullName -Destination (Join-Path $InstallDir "jin.exe") -Force

    # ----------------------------------------------------------------- path

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($null -eq $UserPath) { $UserPath = "" }

    if (($UserPath -split ';') -notcontains $InstallDir) {
        $NewPath = if ($UserPath.Trim() -eq "") { $InstallDir } else { "$UserPath;$InstallDir" }
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        $Added = $true
    } else {
        $Added = $false
    }

    # --------------------------------------------------------------- report

    Write-Host ""
    & (Join-Path $InstallDir "jin.exe") --version
    Write-Host "installed to $(Join-Path $InstallDir 'jin.exe')"

    if ($Added) {
        Write-Host ""
        Write-Host "$InstallDir was added to your user PATH. Open a new terminal for it to take effect."
    } else {
        Write-Host ""
        Write-Host "$InstallDir is already on your PATH."
    }
} finally {
    Remove-Item -Path $Temp -Recurse -Force -ErrorAction SilentlyContinue
}
