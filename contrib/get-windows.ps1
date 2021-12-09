$ErrorActionPreference = 'Stop'
# https://stackoverflow.com/questions/28682642/powershell-why-is-using-invoke-webrequest-much-slower-than-a-browser-download
$ProgressPreference = 'SilentlyContinue'

function Fatal($msg) {
    Write-Error "$msg"
    exit 1
}

function Download($filename) {
    $dist = "https://github.com/wireleap/client/releases/latest/download/"
    Invoke-WebRequest "$dist/$filename" -OutFile "$filename"
}

function Verify-Checksum($binary) {
    $want = (Select-String '[A-Fa-f0-9]{128}' -Path "$binary.hash").Matches.Value.ToUpper()
    $got = (Get-FileHash -Algorithm SHA512 "$binary").Hash
    if (-not ("$want" -eq "$got")) {
        Fatal "sha512 checksum verification failed: expected $want, got $got"
    }
}

<#
.SYNOPSIS
Download, verify and install wireleap client

.DESCRIPTION
This cmdlet will verify your environment's compatibility and download the
latest client binary with the associated hash file to cryptographically verify
its integrity via SHA-512 checksum. If all checks pass, it will release the
binary from quarantine and initialize the client in the specified directory.

.LINK
https://wireleap.com/docs/client/
#>
function Get-Wireleap {
    param(
        # Destination path (eg. $env:USERPROFILE\wireleap, mandatory)
        [Parameter(Mandatory)]
        [string]$Dir,

        # Skip checksum verification (not recommended)
        [Parameter()]
        [switch]$SkipChecksum
    )

    if (Test-Path "$Dir") { Fatal "$Dir already exists" }

    echo "* preparing quarantine ..."
    if (Test-Path "$Dir\quarantine") { rm -r "$Dir\quarantine" }
    mkdir "$Dir\quarantine" > $null
    cd "$Dir\quarantine"

    $bin = "wireleap.exe"
    $fullbin = "wireleap_windows-amd64.exe"
    echo "* downloading $fullbin ..."
    Download "$fullbin"

    echo "* downloading $fullbin.hash ..."
    Download "$fullbin.hash"

    echo "* performing checksum verification on $fullbin ..."
    if (-not($SkipChecksum.IsPresent)) {
        Verify-Checksum("$fullbin")
    } else {
        echo "  SKIPPED!"
    }

    echo "* releasing $fullbin from quarantine ..."
    cd "$Dir"
    Move-Item -Force "quarantine\$fullbin" "$bin"
    rm "quarantine\$fullbin.hash"
    rm -r 'quarantine'

    echo "* performing $binary initialization ..."
    $ErrorActionPreference = 'SilentlyContinue'
    # https://stackoverflow.com/questions/10666101/lastexitcode-0-but-false-in-powershell-redirecting-stderr-to-stdout-gives
    .\wireleap.exe init 2>&1 | %{ "$_" } > out.init
    if ($LASTEXITCODE -ne 0) {
        Fatal ("error\n" + (cat out.init))
    }
    $ErrorActionPreference = 'Stop'
    rm out.init

    echo "* complete"
}
