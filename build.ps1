<#
.SYNOPSIS
    Cross-compilation and release packaging script for TX-UI.

.DESCRIPTION
    Builds TX-UI binaries and optional release packages (.tar.gz / .zip)
    for various OS and architecture targets on a local development machine.

.PARAMETER Target
    Target platform or platforms to build. Can be:
    - "host" or "current" (default): builds for the local host OS/Arch.
    - Specific target (e.g. "linux-amd64", "windows-amd64", "linux-arm64", "darwin-arm64").
    - Comma-separated list (e.g. "linux-amd64,windows-amd64").
    - "all": builds all supported targets.

.PARAMETER Package
    If specified, bundles the binary, service files, Xray core, mtg, and
    geodata files into a release archive (.tar.gz for Linux/macOS, .zip for Windows).

.PARAMETER DownloadAssets
    Downloads Xray-core, mtg-multi, and geodata files for the target platform
    into the staging bin/ directory without creating a compressed archive.

.PARAMETER XrayVersion
    Version tag of Xray-core to bundle (default: "v26.11.9").

.PARAMETER MtgVersion
    Version of mtg-multi to bundle (default: "1.16.0").

.PARAMETER Cgo
    CGO mode: "auto" (default), "1", or "0".
    - "auto": Uses host GCC for host builds; uses cross-GCC or zig if available for cross-compilation;
      otherwise falls back to pure Go (CGO_ENABLED=0).
    - "1": Forces CGO. Requires a compatible C cross-compiler or Zig.
    - "0": Forces pure Go compilation (CGO_ENABLED=0).

.PARAMETER OutputDir
    Root output directory for build artifacts (default: "build").

.PARAMETER Clean
    Cleans the output directory before starting the build.

.PARAMETER Ldflags
    Go linker flags (default: "-w -s").

.PARAMETER Tags
    Additional Go build tags.

.PARAMETER ListTargets
    Prints the list of supported targets and exits.

.EXAMPLE
    .\build.ps1
    Builds the binary for the host machine.

.EXAMPLE
    .\build.ps1 linux-amd64 -Package
    Builds and packages TX-UI for linux-amd64 into build/x-ui-linux-amd64.tar.gz.

.EXAMPLE
    .\build.ps1 -Target linux-amd64,windows-amd64 -Cgo auto
    Cross-compiles binaries for both Linux and Windows.

.EXAMPLE
    .\build.ps1 all -Package
    Builds and packages all supported platforms.
#>

[CmdletBinding()]
param (
    [Parameter(Position = 0)]
    [string[]]$Target = @("host"),

    [switch]$Package,
    [switch]$DownloadAssets,
    [string]$XrayVersion = "v26.11.9",
    [string]$MtgVersion = "1.16.0",
    [ValidateSet("auto", "1", "0")]
    [string]$Cgo = "auto",
    [string]$OutputDir = "build",
    [switch]$Clean,
    [string]$Ldflags = "-w -s",
    [string]$Tags = "",
    [switch]$ListTargets,
    [switch]$Help
)

Set-StrictMode -Off
$ErrorActionPreference = "Stop"

if ($Help) {
    Get-Help $MyInvocation.MyCommand.Path -Detailed
    exit 0
}

# Supported platforms definition
$PlatformMap = [ordered]@{
    "linux-amd64" = @{
        GOOS = "linux"; GOARCH = "amd64"; GOARM = "";
        XrayZip = "Xray-linux-64.zip"; XrayBin = "xray-linux-amd64";
        MtgTar = "mtg-multi-$MtgVersion-linux-amd64.tar.gz"; MtgBin = "mtg-linux-amd64";
        ZigTarget = "x86_64-linux-musl"; GccPrefix = "x86_64-linux-gnu-";
        OutBin = "x-ui"; Archive = "x-ui-linux-amd64.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/amd64", "linux_amd64", "linux64")
    }
    "linux-arm64" = @{
        GOOS = "linux"; GOARCH = "arm64"; GOARM = "";
        XrayZip = "Xray-linux-arm64-v8a.zip"; XrayBin = "xray-linux-arm64";
        MtgTar = "mtg-multi-$MtgVersion-linux-arm64.tar.gz"; MtgBin = "mtg-linux-arm64";
        ZigTarget = "aarch64-linux-musl"; GccPrefix = "aarch64-linux-gnu-";
        OutBin = "x-ui"; Archive = "x-ui-linux-arm64.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/arm64", "linux_arm64", "aarch64", "arm64")
    }
    "linux-armv7" = @{
        GOOS = "linux"; GOARCH = "arm"; GOARM = "7";
        XrayZip = "Xray-linux-arm32-v7a.zip"; XrayBin = "xray-linux-arm";
        MtgTar = "mtg-multi-$MtgVersion-linux-armv7.tar.gz"; MtgBin = "mtg-linux-arm";
        ZigTarget = "arm-linux-musleabihf"; GccPrefix = "arm-linux-gnueabihf-";
        OutBin = "x-ui"; Archive = "x-ui-linux-armv7.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/armv7", "armv7", "arm32-v7a", "linux-arm")
    }
    "linux-armv6" = @{
        GOOS = "linux"; GOARCH = "arm"; GOARM = "6";
        XrayZip = "Xray-linux-arm32-v6.zip"; XrayBin = "xray-linux-arm";
        MtgTar = "mtg-multi-$MtgVersion-linux-armv6.tar.gz"; MtgBin = "mtg-linux-arm";
        ZigTarget = "arm-linux-musleabihf"; GccPrefix = "arm-linux-gnueabihf-";
        OutBin = "x-ui"; Archive = "x-ui-linux-armv6.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/armv6", "armv6", "arm32-v6")
    }
    "linux-armv5" = @{
        GOOS = "linux"; GOARCH = "arm"; GOARM = "5";
        XrayZip = "Xray-linux-arm32-v5.zip"; XrayBin = "xray-linux-arm";
        MtgTar = "mtg-multi-$MtgVersion-linux-armv5.tar.gz"; MtgBin = "mtg-linux-arm";
        ZigTarget = "arm-linux-musleabi"; GccPrefix = "arm-linux-gnueabi-";
        OutBin = "x-ui"; Archive = "x-ui-linux-armv5.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/armv5", "armv5", "arm32-v5")
    }
    "linux-386" = @{
        GOOS = "linux"; GOARCH = "386"; GOARM = "";
        XrayZip = "Xray-linux-32.zip"; XrayBin = "xray-linux-386";
        MtgTar = "mtg-multi-$MtgVersion-linux-386.tar.gz"; MtgBin = "mtg-linux-386";
        ZigTarget = "i386-linux-musl"; GccPrefix = "i686-linux-gnu-";
        OutBin = "x-ui"; Archive = "x-ui-linux-386.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/386", "linux-i386", "linux-32")
    }
    "linux-s390x" = @{
        GOOS = "linux"; GOARCH = "s390x"; GOARM = "";
        XrayZip = "Xray-linux-s390x.zip"; XrayBin = "xray-linux-s390x";
        MtgTar = "mtg-multi-$MtgVersion-linux-s390x.tar.gz"; MtgBin = "mtg-linux-s390x";
        ZigTarget = "s390x-linux-musl"; GccPrefix = "s390x-linux-gnu-";
        OutBin = "x-ui"; Archive = "x-ui-linux-s390x.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/s390x", "s390x")
    }
    "windows-amd64" = @{
        GOOS = "windows"; GOARCH = "amd64"; GOARM = "";
        XrayZip = "Xray-windows-64.zip"; XrayBin = "xray-windows-amd64.exe";
        MtgZip = "mtg-multi-$MtgVersion-windows-amd64.zip"; MtgBin = "mtg-windows-amd64.exe";
        ZigTarget = "x86_64-windows-gnu"; GccPrefix = "x86_64-w64-mingw32-";
        OutBin = "x-ui.exe"; Archive = "x-ui-windows-amd64.zip"; Format = "zip";
        Aliases = @("windows/amd64", "windows-64", "win64", "windows")
    }
    "windows-arm64" = @{
        GOOS = "windows"; GOARCH = "arm64"; GOARM = "";
        XrayZip = "Xray-windows-arm64-v8a.zip"; XrayBin = "xray-windows-arm64.exe";
        MtgZip = "mtg-multi-$MtgVersion-windows-arm64.zip"; MtgBin = "mtg-windows-arm64.exe";
        ZigTarget = "aarch64-windows-gnu"; GccPrefix = "aarch64-w64-mingw32-";
        OutBin = "x-ui.exe"; Archive = "x-ui-windows-arm64.zip"; Format = "zip";
        Aliases = @("windows/arm64", "win-arm64")
    }
    "windows-386" = @{
        GOOS = "windows"; GOARCH = "386"; GOARM = "";
        XrayZip = "Xray-windows-32.zip"; XrayBin = "xray-windows-386.exe";
        MtgZip = "mtg-multi-$MtgVersion-windows-386.zip"; MtgBin = "mtg-windows-386.exe";
        ZigTarget = "i386-windows-gnu"; GccPrefix = "i686-w64-mingw32-";
        OutBin = "x-ui.exe"; Archive = "x-ui-windows-386.zip"; Format = "zip";
        Aliases = @("windows/386", "windows-32", "win32")
    }
    "darwin-amd64" = @{
        GOOS = "darwin"; GOARCH = "amd64"; GOARM = "";
        XrayZip = "Xray-macos-64.zip"; XrayBin = "xray-darwin-amd64";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "x86_64-macos-none"; GccPrefix = "o64-clang";
        OutBin = "x-ui"; Archive = "x-ui-darwin-amd64.tar.gz"; Format = "tar.gz";
        Aliases = @("darwin/amd64", "macos-amd64", "mac-amd64")
    }
    "darwin-arm64" = @{
        GOOS = "darwin"; GOARCH = "arm64"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-macos-arm64-v8a.zip"; XrayBin = "xray-darwin-arm64";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "aarch64-macos-none"; GccPrefix = "oa64-clang";
        OutBin = "x-ui"; Archive = "x-ui-darwin-arm64.tar.gz"; Format = "tar.gz";
        Aliases = @("darwin/arm64", "macos-arm64", "mac-arm64")
    }
    "linux-riscv64" = @{
        GOOS = "linux"; GOARCH = "riscv64"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-linux-riscv64.zip"; XrayBin = "xray-linux-riscv64";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "riscv64-linux-musl"; GccPrefix = "riscv64-linux-gnu-";
        OutBin = "x-ui"; Archive = "x-ui-linux-riscv64.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/riscv64", "riscv64")
    }
    "linux-mips" = @{
        GOOS = "linux"; GOARCH = "mips"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-linux-mips32.zip"; XrayBin = "xray-linux-mips";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "mips-linux-musl"; GccPrefix = "mips-linux-gnu-";
        OutBin = "x-ui"; Archive = "x-ui-linux-mips.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/mips", "mips", "mips32")
    }
    "linux-mipsle" = @{
        GOOS = "linux"; GOARCH = "mipsle"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-linux-mips32le.zip"; XrayBin = "xray-linux-mipsle";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "mipsel-linux-musl"; GccPrefix = "mipsel-linux-gnu-";
        OutBin = "x-ui"; Archive = "x-ui-linux-mipsle.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/mipsle", "mipsle", "mips32le")
    }
    "linux-mips64" = @{
        GOOS = "linux"; GOARCH = "mips64"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-linux-mips64.zip"; XrayBin = "xray-linux-mips64";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "mips64-linux-musl"; GccPrefix = "mips64-linux-gnuabi64-";
        OutBin = "x-ui"; Archive = "x-ui-linux-mips64.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/mips64", "mips64")
    }
    "linux-mips64le" = @{
        GOOS = "linux"; GOARCH = "mips64le"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-linux-mips64le.zip"; XrayBin = "xray-linux-mips64le";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "mips64el-linux-musl"; GccPrefix = "mips64el-linux-gnuabi64-";
        OutBin = "x-ui"; Archive = "x-ui-linux-mips64le.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/mips64le", "mips64le")
    }
    "linux-ppc64le" = @{
        GOOS = "linux"; GOARCH = "ppc64le"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-linux-ppc64le.zip"; XrayBin = "xray-linux-ppc64le";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "powerpc64le-linux-musl"; GccPrefix = "powerpc64le-linux-gnu-";
        OutBin = "x-ui"; Archive = "x-ui-linux-ppc64le.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/ppc64le", "ppc64le", "powerpc64le")
    }
    "freebsd-amd64" = @{
        GOOS = "freebsd"; GOARCH = "amd64"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-freebsd-64.zip"; XrayBin = "xray-freebsd-amd64";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "x86_64-freebsd"; GccPrefix = "x86_64-unknown-freebsd-";
        OutBin = "x-ui"; Archive = "x-ui-freebsd-amd64.tar.gz"; Format = "tar.gz";
        Aliases = @("freebsd/amd64", "freebsd-64", "freebsd")
    }
    "freebsd-arm64" = @{
        GOOS = "freebsd"; GOARCH = "arm64"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-freebsd-arm64-v8a.zip"; XrayBin = "xray-freebsd-arm64";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "aarch64-freebsd"; GccPrefix = "aarch64-unknown-freebsd-";
        OutBin = "x-ui"; Archive = "x-ui-freebsd-arm64.tar.gz"; Format = "tar.gz";
        Aliases = @("freebsd/arm64", "freebsd-arm")
    }
    "freebsd-386" = @{
        GOOS = "freebsd"; GOARCH = "386"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-freebsd-32.zip"; XrayBin = "xray-freebsd-386";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "i386-freebsd"; GccPrefix = "i686-unknown-freebsd-";
        OutBin = "x-ui"; Archive = "x-ui-freebsd-386.tar.gz"; Format = "tar.gz";
        Aliases = @("freebsd/386", "freebsd-32")
    }
    "linux-loong64" = @{
        GOOS = "linux"; GOARCH = "loong64"; GOARM = ""; GOMIPS = "";
        XrayZip = "Xray-linux-loong64.zip"; XrayBin = "xray-linux-loong64";
        MtgTar = ""; MtgBin = "";
        ZigTarget = "loongarch64-linux-musl"; GccPrefix = "loongarch64-linux-gnu-";
        OutBin = "x-ui"; Archive = "x-ui-linux-loong64.tar.gz"; Format = "tar.gz";
        Aliases = @("linux/loong64", "loong64")
    }
}

if ($ListTargets) {
    Write-Host "`nSupported Targets:" -ForegroundColor Cyan
    foreach ($key in $PlatformMap.Keys) {
        $aliases = ($PlatformMap[$key].Aliases -join ", ")
        Write-Host ("  {0,-16} (Aliases: {1})" -f $key, $aliases)
    }
    Write-Host "`nSpecial targets: 'host' / 'current' (default), 'all'`n" -ForegroundColor DarkGray
    exit 0
}

# Resolve Host OS and Arch
function Get-HostPlatform {
    $os = "linux"
    if ($PSVersionTable.PSVersion.Major -le 5 -or $IsWindows -or ($env:OS -like "*Windows*")) {
        $os = "windows"
    } elseif ($IsMacOS) {
        $os = "darwin"
    }

    $arch = "amd64"
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        $arch = "arm64"
    } elseif ($env:PROCESSOR_ARCHITECTURE -match "x86|386" -and -not [System.Environment]::Is64BitOperatingSystem) {
        $arch = "386"
    }

    return "$os-$arch"
}

$HostPlatform = Get-HostPlatform

# Normalize target list
$ResolvedTargets = [System.Collections.Generic.List[string]]::new()
foreach ($entry in $Target) {
    $tokens = $entry -split "[,;\s]+"
    foreach ($token in $tokens) {
        $t = $token.Trim().ToLower()
        if (-not $t) { continue }
        if ($t -eq "host" -or $t -eq "current") {
            $t = $HostPlatform
        }
        if ($t -eq "all") {
            foreach ($k in $PlatformMap.Keys) {
                if (-not $ResolvedTargets.Contains($k)) {
                    $ResolvedTargets.Add($k)
                }
            }
            continue
        }

        # Match alias or exact key
        $matched = $null
        foreach ($k in $PlatformMap.Keys) {
            if ($k -eq $t -or $PlatformMap[$k].Aliases -contains $t) {
                $matched = $k
                break
            }
        }
        if ($matched) {
            if (-not $ResolvedTargets.Contains($matched)) {
                $ResolvedTargets.Add($matched)
            }
        } else {
            Write-Warning "Unknown target platform: '$token'. Skipping."
        }
    }
}

if ($ResolvedTargets.Count -eq 0) {
    Write-Error "No valid targets specified to build."
    exit 1
}

# Determine C Compiler
function Resolve-Compiler {
    param(
        [hashtable]$Info,
        [string]$TargetKey
    )

    if ($Cgo -eq "0") {
        return @{ CgoEnabled = "0"; CC = $null; Note = "Pure Go (CGO disabled)" }
    }

    if ($env:CC) {
        return @{ CgoEnabled = "1"; CC = $env:CC; Note = "Using CC from environment: $env:CC" }
    }

    # Host build
    if ($TargetKey -eq $HostPlatform) {
        if (Get-Command gcc -ErrorAction SilentlyContinue) {
            return @{ CgoEnabled = "1"; CC = "gcc"; Note = "Host GCC" }
        }
        if (Get-Command clang -ErrorAction SilentlyContinue) {
            return @{ CgoEnabled = "1"; CC = "clang"; Note = "Host Clang" }
        }
    }

    # Target-specific cross-compiler
    if ($Info.GccPrefix) {
        $crossGcc = "$($Info.GccPrefix)gcc"
        if (Get-Command $crossGcc -ErrorAction SilentlyContinue) {
            return @{ CgoEnabled = "1"; CC = $crossGcc; Note = "Target GCC: $crossGcc" }
        }
    }

    # Zig cross-compiler
    if (Get-Command zig -ErrorAction SilentlyContinue) {
        if ($Info.ZigTarget) {
            return @{ CgoEnabled = "1"; CC = "zig cc -target $($Info.ZigTarget)"; Note = "Zig CC ($($Info.ZigTarget))" }
        }
    }

    # If CGO was explicitly required
    if ($Cgo -eq "1") {
        throw "CGO is required (-Cgo 1) for $TargetKey but no cross-compiler was found. Install $($Info.GccPrefix)gcc or install zig."
    }

    # Fallback to pure Go
    return @{ CgoEnabled = "0"; CC = $null; Note = "Pure Go fallback (CGO_ENABLED=0 with pure-Go SQLite)" }
}

# Asset Downloader with Local Cache
$CacheDir = Join-Path $OutputDir ".cache"
$null = New-Item -ItemType Directory -Path $CacheDir -Force

function Get-CachedAsset {
    param(
        [string]$Url,
        [string]$SubDir,
        [string]$FileName
    )

    $destFolder = Join-Path $CacheDir $SubDir
    $null = New-Item -ItemType Directory -Path $destFolder -Force
    $destFile = Join-Path $destFolder $FileName

    if (Test-Path $destFile) {
        $size = (Get-Item $destFile).Length
        if ($size -gt 0) {
            Write-Verbose "Using cached asset: $destFile"
            return $destFile
        }
    }

    # Check local bin/ folder as pre-existing source
    $localBinFile = Join-Path "bin" $FileName
    if (Test-Path $localBinFile) {
        $size = (Get-Item $localBinFile).Length
        if ($size -gt 0) {
            Copy-Item $localBinFile $destFile -Force
            return $destFile
        }
    }

    Write-Host "  -> Downloading: $Url" -ForegroundColor DarkGray
    try {
        Invoke-WebRequest -Uri $Url -OutFile $destFile -TimeoutSec 120
    } catch {
        if (Test-Path $destFile) { Remove-Item $destFile -Force -ErrorAction SilentlyContinue }
        if (Test-Path $localBinFile) {
            Write-Warning "Download failed ($Url), using local bin/ fallback: $localBinFile"
            Copy-Item $localBinFile $destFile -Force
            return $destFile
        }
        throw "Failed to download $($Url): $_"
    }

    return $destFile
}

# Clean if requested
if ($Clean) {
    if (Test-Path $OutputDir) {
        Write-Host "Cleaning output directory: $OutputDir" -ForegroundColor Yellow
        # Keep .cache if it exists to preserve downloaded assets
        $savedCache = $null
        if (Test-Path $CacheDir) {
            $savedCache = Join-Path $env:TEMP ("txui-cache-" + [System.Guid]::NewGuid().ToString("N"))
            Move-Item $CacheDir $savedCache -Force
        }
        Remove-Item $OutputDir -Recurse -Force
        $null = New-Item -ItemType Directory -Path $OutputDir -Force
        if ($savedCache -and (Test-Path $savedCache)) {
            Move-Item $savedCache $CacheDir -Force
        }
    }
} else {
    $null = New-Item -ItemType Directory -Path $OutputDir -Force
}

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " TX-UI Cross Build Script" -ForegroundColor Cyan
Write-Host " Host Platform : $HostPlatform" -ForegroundColor DarkGray
Write-Host " Targets       : $($ResolvedTargets -join ', ')" -ForegroundColor DarkGray
Write-Host " Package Mode  : $(if ($Package) { 'Yes (Full release archive)' } else { 'No (Binary only)' })" -ForegroundColor DarkGray
Write-Host " Output Dir    : $OutputDir" -ForegroundColor DarkGray
Write-Host "==========================================================" -ForegroundColor Cyan

$SuccessCount = 0
$FailCount = 0

foreach ($targetKey in $ResolvedTargets) {
    $info = $PlatformMap[$targetKey]
    Write-Host "`n>>> Building target: $targetKey ($($info.GOOS)/$($info.GOARCH))" -ForegroundColor Green

    $compiler = Resolve-Compiler -Info $info -TargetKey $targetKey
    Write-Host "  Compiler  : $($compiler.Note)" -ForegroundColor DarkGray

    # Set Go environment variables
    $env:GOOS = $info.GOOS
    $env:GOARCH = $info.GOARCH
    if ($info.GOARM) {
        $env:GOARM = $info.GOARM
    } else {
        Remove-Item Env:\GOARM -ErrorAction SilentlyContinue
    }
    if ($info.GOMIPS) {
        $env:GOMIPS = $info.GOMIPS
    } else {
        Remove-Item Env:\GOMIPS -ErrorAction SilentlyContinue
    }
    $env:CGO_ENABLED = $compiler.CgoEnabled
    if ($compiler.CC) {
        $env:CC = $compiler.CC
    } else {
        Remove-Item Env:\CC -ErrorAction SilentlyContinue
    }

    # Staging paths
    $targetDir = Join-Path $OutputDir $targetKey
    $stagingDir = Join-Path $targetDir "x-ui"
    $binDir = Join-Path $stagingDir "bin"
    $null = New-Item -ItemType Directory -Path $binDir -Force

    $binaryPath = Join-Path $stagingDir $info.OutBin
    $ext = if ($info.GOOS -eq "windows") { ".exe" } else { "" }
    $directBinPath = Join-Path $OutputDir "x-ui-$targetKey$ext"

    # Build Go binary
    $buildArgs = @("build", "-ldflags", $Ldflags)
    if ($Tags) {
        $buildArgs += @("-tags", $Tags)
    }
    $buildArgs += @("-o", $binaryPath, "main.go")

    Write-Host "  -> Running: go $($buildArgs -join ' ')" -ForegroundColor DarkCyan
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        & go @buildArgs
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed with exit code $LASTEXITCODE"
        }
        $sw.Stop()
        Write-Host ("  [OK] Binary built in {0:N1}s ({1} MB)" -f $sw.Elapsed.TotalSeconds, [math]::Round(((Get-Item $binaryPath).Length / 1MB), 2)) -ForegroundColor Green

        # Copy standalone binary to root output dir for quick access
        Copy-Item -Path $binaryPath -Destination $directBinPath -Force
    } catch {
        $sw.Stop()
        Write-Host "  [FAIL] Compilation failed: $_" -ForegroundColor Red
        $FailCount++
        continue
    }

    # Download Assets & Package if requested
    if ($Package -or $DownloadAssets) {
        Write-Host "  -> Fetching runtime dependencies..." -ForegroundColor DarkCyan

        # 1. Xray-core
        if ($info.XrayZip) {
            try {
                $xrayUrl = "https://github.com/XTLS/Xray-core/releases/download/$XrayVersion/$($info.XrayZip)"
                $cachedZip = Get-CachedAsset -Url $xrayUrl -SubDir "xray/$XrayVersion" -FileName $info.XrayZip
                
                # Unzip into temp folder
                $tempExtract = Join-Path $targetDir "_xray_tmp"
                $null = New-Item -ItemType Directory -Path $tempExtract -Force
                Expand-Archive -Path $cachedZip -DestinationPath $tempExtract -Force
                
                # Copy xray binary
                $srcXray = if ($info.GOOS -eq "windows") { Join-Path $tempExtract "xray.exe" } else { Join-Path $tempExtract "xray" }
                if (Test-Path $srcXray) {
                    $destXray = Join-Path $binDir $info.XrayBin
                    Copy-Item $srcXray $destXray -Force
                }
                Remove-Item $tempExtract -Recurse -Force -ErrorAction SilentlyContinue
                Write-Host "  [OK] Xray-core ($XrayVersion) staged" -ForegroundColor DarkGreen
            } catch {
                Write-Warning "Failed to bundle Xray-core: $_"
            }
        }

        # 2. mtg-multi
        $mtgFile = if ($info.MtgZip) { $info.MtgZip } else { $info.MtgTar }
        if ($mtgFile -and $info.MtgBin) {
            try {
                $mtgUrl = "https://github.com/mhsanaei/mtg-multi/releases/download/v$MtgVersion/$mtgFile"
                $cachedMtg = Get-CachedAsset -Url $mtgUrl -SubDir "mtg/$MtgVersion" -FileName $mtgFile
                $tempMtg = Join-Path $targetDir "_mtg_tmp"
                $null = New-Item -ItemType Directory -Path $tempMtg -Force

                if ($info.MtgZip) {
                    Expand-Archive -Path $cachedMtg -DestinationPath $tempMtg -Force
                    $srcMtg = Get-ChildItem -Path $tempMtg -Filter "mtg-multi.exe" -Recurse | Select-Object -First 1
                    if ($srcMtg) {
                        Copy-Item $srcMtg.FullName (Join-Path $binDir $info.MtgBin) -Force
                    }
                } else {
                    if (Get-Command tar -ErrorAction SilentlyContinue) {
                        tar -xzf $cachedMtg -C $tempMtg 2>$null
                        $srcMtg = Get-ChildItem -Path $tempMtg -Filter "mtg-multi" -Recurse | Select-Object -First 1
                        if ($srcMtg) {
                            Copy-Item $srcMtg.FullName (Join-Path $binDir $info.MtgBin) -Force
                        }
                    }
                }
                Remove-Item $tempMtg -Recurse -Force -ErrorAction SilentlyContinue
                Write-Host "  [OK] mtg-multi (v$MtgVersion) staged" -ForegroundColor DarkGreen
            } catch {
                Write-Warning "Failed to bundle mtg-multi: $_"
            }
        }

        # 3. Geodata files
        try {
            $geoUrls = @{
                "geoip.dat"      = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
                "geosite.dat"    = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
                "geoip_IR.dat"   = "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat"
                "geosite_IR.dat" = "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat"
                "geoip_RU.dat"   = "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat"
                "geosite_RU.dat" = "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat"
            }
            foreach ($geoName in $geoUrls.Keys) {
                $cachedGeo = Get-CachedAsset -Url $geoUrls[$geoName] -SubDir "geo" -FileName $geoName
                Copy-Item $cachedGeo (Join-Path $binDir $geoName) -Force
            }
            # Add version tags
            $versionContent = "latest-$(Get-Date -Format 'yyyyMMdd')"
            Set-Content -Path (Join-Path $binDir "geoip.dat.version") -Value $versionContent -NoNewline
            Set-Content -Path (Join-Path $binDir "geosite.dat.version") -Value $versionContent -NoNewline
            Write-Host "  [OK] Geodata rules staged" -ForegroundColor DarkGreen
        } catch {
            Write-Warning "Failed to bundle geodata: $_"
        }

        # 4. Copy scripts and service units
        if (Test-Path "x-ui.sh") { Copy-Item "x-ui.sh" $stagingDir -Force }
        if (Test-Path "x-ui.service") { Copy-Item "x-ui.service" $stagingDir -Force }
        if ($info.GOOS -eq "windows") {
            if (Test-Path "bin/wintun.dll") { Copy-Item "bin/wintun.dll" $binDir -Force }
            if (Test-Path "bin/xray_no_window.ps1") { Copy-Item "bin/xray_no_window.ps1" $binDir -Force }
            if (Test-Path "bin/xray_no_window.vbs") { Copy-Item "bin/xray_no_window.vbs" $binDir -Force }
        }

        # 5. Archive package
        if ($Package) {
            $archivePath = [System.IO.Path]::GetFullPath((Join-Path $OutputDir $info.Archive))
            if (Test-Path $archivePath) { Remove-Item $archivePath -Force }
            Write-Host "  -> Packaging archive: $archivePath" -ForegroundColor DarkCyan

            if ($info.Format -eq "zip") {
                if (Get-Command Compress-Archive -ErrorAction SilentlyContinue) {
                    Compress-Archive -Path $stagingDir -DestinationPath $archivePath -Force
                } elseif (Get-Command 7z -ErrorAction SilentlyContinue) {
                    & 7z a -tzip $archivePath "$stagingDir\*" | Out-Null
                }
            } else {
                # tar.gz
                if (Get-Command tar -ErrorAction SilentlyContinue) {
                    Push-Location $targetDir
                    try {
                        & tar -czf $archivePath "x-ui"
                    } finally {
                        Pop-Location
                    }
                } elseif (Get-Command 7z -ErrorAction SilentlyContinue) {
                    $tarFile = $archivePath -replace '\.gz$', ''
                    & 7z a -ttar $tarFile "$stagingDir\*" | Out-Null
                    & 7z a -tgzip $archivePath $tarFile | Out-Null
                    Remove-Item $tarFile -Force -ErrorAction SilentlyContinue
                } else {
                    Write-Warning "Neither 'tar' nor '7z' found in PATH. Archive creation skipped."
                }
            }

            if (Test-Path $archivePath) {
                Write-Host ("  [OK] Package created: {0} ({1} MB)" -f (Split-Path $archivePath -Leaf), [math]::Round(((Get-Item $archivePath).Length / 1MB), 2)) -ForegroundColor Green
            }
        }
    }

    $SuccessCount++
}

# Restore host environment
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\GOARM -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
Remove-Item Env:\CC -ErrorAction SilentlyContinue

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host (" Build Complete! Succeeded: {0}, Failed: {1}" -f $SuccessCount, $FailCount) -ForegroundColor $(if ($FailCount -eq 0) { "Green" } else { "Yellow" })
Write-Host " Artifacts location: $OutputDir" -ForegroundColor DarkGray
Write-Host "==========================================================`n" -ForegroundColor Cyan

if ($FailCount -gt 0) {
    exit 1
}
