#!/usr/bin/env bash
#
# Cross-compilation and release packaging script for TX-UI.
# Supports Linux, macOS, WSL, and Git Bash.
#

set -euo pipefail

# Default configuration
TARGETS=("host")
PACKAGE=0
DOWNLOAD_ASSETS=0
XRAY_VERSION="v26.11.9"
MTG_VERSION="1.16.0"
CGO_MODE="auto"
OUTPUT_DIR="build"
CLEAN=0
LDFLAGS="-w -s"
TAGS=""

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

show_help() {
    cat <<EOF
Usage: $0 [targets...] [options]

Arguments:
  [targets...]               One or more target platforms (e.g. linux-amd64, windows-amd64, all, host).
                             Default: "host" (current machine).

Options:
  -p, --package              Bundle the binary, service files, Xray-core, mtg, and geodata into release archive.
  --download-assets          Download runtime assets into staging bin/ without creating a compressed archive.
  --xray-version <version>   Xray-core release version to bundle (default: v26.11.9).
  --mtg-version <version>    mtg-multi release version to bundle (default: 1.16.0).
  --cgo <auto|1|0>           CGO compilation mode (default: auto).
                             - auto: Use host compiler for host build; cross-compiler/zig if available; else pure Go.
                             - 1: Force CGO (requires cross-compiler or Zig).
                             - 0: Force pure Go (CGO_ENABLED=0).
  -o, --output-dir <dir>     Output directory for built artifacts (default: build).
  --clean                    Clean output directory before building.
  --ldflags <flags>          Go linker flags (default: "-w -s").
  --tags <tags>              Go build tags.
  --list-targets             Print all supported target platforms and exit.
  -h, --help                 Show this help message and exit.

Examples:
  $0                         Build for current host platform
  $0 linux-amd64 -p          Build and package release archive for linux-amd64
  $0 linux-amd64 windows-amd64 --cgo auto
  $0 all --package           Build and package all supported platforms
EOF
}

# Target platforms table
# Format: key | goos | goarch | goarm | xray_zip | xray_bin | mtg_file | mtg_bin | zig_target | gcc_prefix | out_bin | archive | format
get_target_info() {
    case "$1" in
        linux-amd64|linux/amd64|linux_amd64|linux64)
            echo "linux-amd64|linux|amd64||Xray-linux-64.zip|xray-linux-amd64|mtg-multi-${MTG_VERSION}-linux-amd64.tar.gz|mtg-linux-amd64|x86_64-linux-musl|x86_64-linux-gnu-|x-ui|x-ui-linux-amd64.tar.gz|tar.gz"
            ;;
        linux-arm64|linux/arm64|linux_arm64|aarch64|arm64)
            echo "linux-arm64|linux|arm64||Xray-linux-arm64-v8a.zip|xray-linux-arm64|mtg-multi-${MTG_VERSION}-linux-arm64.tar.gz|mtg-linux-arm64|aarch64-linux-musl|aarch64-linux-gnu-|x-ui|x-ui-linux-arm64.tar.gz|tar.gz"
            ;;
        linux-armv7|linux/armv7|armv7|arm32-v7a|linux-arm)
            echo "linux-armv7|linux|arm|7|Xray-linux-arm32-v7a.zip|xray-linux-arm|mtg-multi-${MTG_VERSION}-linux-armv7.tar.gz|mtg-linux-arm|arm-linux-musleabihf|arm-linux-gnueabihf-|x-ui|x-ui-linux-armv7.tar.gz|tar.gz"
            ;;
        linux-armv6|linux/armv6|armv6|arm32-v6)
            echo "linux-armv6|linux|arm|6|Xray-linux-arm32-v6.zip|xray-linux-arm|mtg-multi-${MTG_VERSION}-linux-armv6.tar.gz|mtg-linux-arm|arm-linux-musleabihf|arm-linux-gnueabihf-|x-ui|x-ui-linux-armv6.tar.gz|tar.gz"
            ;;
        linux-armv5|linux/armv5|armv5|arm32-v5)
            echo "linux-armv5|linux|arm|5|Xray-linux-arm32-v5.zip|xray-linux-arm|mtg-multi-${MTG_VERSION}-linux-armv5.tar.gz|mtg-linux-arm|arm-linux-musleabi|arm-linux-gnueabi-|x-ui|x-ui-linux-armv5.tar.gz|tar.gz"
            ;;
        linux-386|linux/386|linux-i386|linux-32)
            echo "linux-386|linux|386||Xray-linux-32.zip|xray-linux-386|mtg-multi-${MTG_VERSION}-linux-386.tar.gz|mtg-linux-386|i386-linux-musl|i686-linux-gnu-|x-ui|x-ui-linux-386.tar.gz|tar.gz"
            ;;
        linux-s390x|linux/s390x|s390x)
            echo "linux-s390x|linux|s390x||Xray-linux-s390x.zip|xray-linux-s390x|mtg-multi-${MTG_VERSION}-linux-s390x.tar.gz|mtg-linux-s390x|s390x-linux-musl|s390x-linux-gnu-|x-ui|x-ui-linux-s390x.tar.gz|tar.gz"
            ;;
        windows-amd64|windows/amd64|windows-64|win64|windows)
            echo "windows-amd64|windows|amd64||Xray-windows-64.zip|xray-windows-amd64.exe|mtg-multi-${MTG_VERSION}-windows-amd64.zip|mtg-windows-amd64.exe|x86_64-windows-gnu|x86_64-w64-mingw32-|x-ui.exe|x-ui-windows-amd64.zip|zip"
            ;;
        windows-arm64|windows/arm64|win-arm64)
            echo "windows-arm64|windows|arm64||Xray-windows-arm64-v8a.zip|xray-windows-arm64.exe|mtg-multi-${MTG_VERSION}-windows-arm64.zip|mtg-windows-arm64.exe|aarch64-windows-gnu|aarch64-w64-mingw32-|x-ui.exe|x-ui-windows-arm64.zip|zip"
            ;;
        windows-386|windows/386|windows-32|win32)
            echo "windows-386|windows|386||Xray-windows-32.zip|xray-windows-386.exe|mtg-multi-${MTG_VERSION}-windows-386.zip|mtg-windows-386.exe|i386-windows-gnu|i686-w64-mingw32-|x-ui.exe|x-ui-windows-386.zip|zip"
            ;;
        darwin-amd64|darwin/amd64|macos-amd64|mac-amd64)
            echo "darwin-amd64|darwin|amd64||Xray-macos-64.zip|xray-darwin-amd64|||x86_64-macos-none|o64-clang|x-ui|x-ui-darwin-amd64.tar.gz|tar.gz|"
            ;;
        darwin-arm64|darwin/arm64|macos-arm64|mac-arm64)
            echo "darwin-arm64|darwin|arm64||Xray-macos-arm64-v8a.zip|xray-darwin-arm64|||aarch64-macos-none|oa64-clang|x-ui|x-ui-darwin-arm64.tar.gz|tar.gz|"
            ;;
        linux-riscv64|linux/riscv64|riscv64)
            echo "linux-riscv64|linux|riscv64||Xray-linux-riscv64.zip|xray-linux-riscv64|||riscv64-linux-musl|riscv64-linux-gnu-|x-ui|x-ui-linux-riscv64.tar.gz|tar.gz|"
            ;;
        linux-mips|linux/mips|mips|mips32)
            echo "linux-mips|linux|mips||Xray-linux-mips32.zip|xray-linux-mips|||mips-linux-musl|mips-linux-gnu-|x-ui|x-ui-linux-mips.tar.gz|tar.gz|"
            ;;
        linux-mipsle|linux/mipsle|mipsle|mips32le)
            echo "linux-mipsle|linux|mipsle||Xray-linux-mips32le.zip|xray-linux-mipsle|||mipsel-linux-musl|mipsel-linux-gnu-|x-ui|x-ui-linux-mipsle.tar.gz|tar.gz|"
            ;;
        linux-mips64|linux/mips64|mips64)
            echo "linux-mips64|linux|mips64||Xray-linux-mips64.zip|xray-linux-mips64|||mips64-linux-musl|mips64-linux-gnuabi64-|x-ui|x-ui-linux-mips64.tar.gz|tar.gz|"
            ;;
        linux-mips64le|linux/mips64le|mips64le)
            echo "linux-mips64le|linux|mips64le||Xray-linux-mips64le.zip|xray-linux-mips64le|||mips64el-linux-musl|mips64el-linux-gnuabi64-|x-ui|x-ui-linux-mips64le.tar.gz|tar.gz|"
            ;;
        linux-ppc64le|linux/ppc64le|ppc64le|powerpc64le)
            echo "linux-ppc64le|linux|ppc64le||Xray-linux-ppc64le.zip|xray-linux-ppc64le|||powerpc64le-linux-musl|powerpc64le-linux-gnu-|x-ui|x-ui-linux-ppc64le.tar.gz|tar.gz|"
            ;;
        freebsd-amd64|freebsd/amd64|freebsd-64|freebsd)
            echo "freebsd-amd64|freebsd|amd64||Xray-freebsd-64.zip|xray-freebsd-amd64|||x86_64-freebsd|x86_64-unknown-freebsd-|x-ui|x-ui-freebsd-amd64.tar.gz|tar.gz|"
            ;;
        freebsd-arm64|freebsd/arm64|freebsd-arm)
            echo "freebsd-arm64|freebsd|arm64||Xray-freebsd-arm64-v8a.zip|xray-freebsd-arm64|||aarch64-freebsd|aarch64-unknown-freebsd-|x-ui|x-ui-freebsd-arm64.tar.gz|tar.gz|"
            ;;
        freebsd-386|freebsd/386|freebsd-32)
            echo "freebsd-386|freebsd|386||Xray-freebsd-32.zip|xray-freebsd-386|||i386-freebsd|i686-unknown-freebsd-|x-ui|x-ui-freebsd-386.tar.gz|tar.gz|"
            ;;
        linux-loong64|linux/loong64|loong64)
            echo "linux-loong64|linux|loong64||Xray-linux-loong64.zip|xray-linux-loong64|||loongarch64-linux-musl|loongarch64-linux-gnu-|x-ui|x-ui-linux-loong64.tar.gz|tar.gz|"
            ;;
        *)
            echo ""
            ;;
    esac
}

ALL_PLATFORMS=(
    "linux-amd64" "linux-arm64" "linux-armv7" "linux-armv6" "linux-armv5" "linux-386" "linux-s390x"
    "windows-amd64" "windows-arm64" "windows-386"
    "darwin-amd64" "darwin-arm64"
    "linux-riscv64" "linux-mips" "linux-mipsle" "linux-mips64" "linux-mips64le" "linux-ppc64le"
    "freebsd-amd64" "freebsd-arm64" "freebsd-386" "linux-loong64"
)

# Detect host platform
detect_host_platform() {
    local os=""
    local arch=""
    case "$(uname -s)" in
        Linux*)   os="linux" ;;
        Darwin*)  os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *)        os="linux" ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        armv7*) arch="armv7" ;;
        armv6*) arch="armv6" ;;
        i386|i686) arch="386" ;;
        s390x) arch="s390x" ;;
        *) arch="amd64" ;;
    esac

    echo "${os}-${arch}"
}

HOST_PLATFORM=$(detect_host_platform)

# Parse CLI arguments
POSITIONAL_ARGS=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        -p|--package)
            PACKAGE=1
            shift
            ;;
        --download-assets)
            DOWNLOAD_ASSETS=1
            shift
            ;;
        --xray-version)
            XRAY_VERSION="$2"
            shift 2
            ;;
        --mtg-version)
            MTG_VERSION="$2"
            shift 2
            ;;
        --cgo)
            CGO_MODE="$2"
            shift 2
            ;;
        -o|--output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --clean)
            CLEAN=1
            shift
            ;;
        --ldflags)
            LDFLAGS="$2"
            shift 2
            ;;
        --tags)
            TAGS="$2"
            shift 2
            ;;
        --list-targets)
            echo -e "${CYAN}Supported Targets:${NC}"
            for p in "${ALL_PLATFORMS[@]}"; do
                echo "  $p"
            done
            echo -e "${GRAY}Special targets: host (default), all${NC}"
            exit 0
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        -*)
            echo -e "${RED}Error: Unknown option $1${NC}" >&2
            show_help
            exit 1
            ;;
        *)
            POSITIONAL_ARGS+=("$1")
            shift
            ;;
    esac
done

if [[ ${#POSITIONAL_ARGS[@]} -gt 0 ]]; then
    TARGETS=("${POSITIONAL_ARGS[@]}")
fi

# Resolve target list
RESOLVED_TARGETS=()
for t in "${TARGETS[@]}"; do
    IFS=',' read -ra ADDR <<< "$t"
    for item in "${ADDR[@]}"; do
        item=$(echo "$item" | tr '[:upper:]' '[:lower:]' | xargs)
        if [[ -z "$item" ]]; then continue; fi
        if [[ "$item" == "host" || "$item" == "current" ]]; then
            item="$HOST_PLATFORM"
        fi
        if [[ "$item" == "all" ]]; then
            for p in "${ALL_PLATFORMS[@]}"; do
                RESOLVED_TARGETS+=("$p")
            done
            continue
        fi

        info=$(get_target_info "$item")
        if [[ -n "$info" ]]; then
            canonical=$(echo "$info" | cut -d'|' -f1)
            RESOLVED_TARGETS+=("$canonical")
        else
            echo -e "${YELLOW}Warning: Unknown target '$item'. Skipping.${NC}" >&2
        fi
    done
done

# Deduplicate targets
UNIQUE_TARGETS=($(printf "%s\n" "${RESOLVED_TARGETS[@]}" | sort -u))

if [[ ${#UNIQUE_TARGETS[@]} -eq 0 ]]; then
    echo -e "${RED}Error: No valid targets specified.${NC}" >&2
    exit 1
fi

CACHE_DIR="${OUTPUT_DIR}/.cache"
mkdir -p "$CACHE_DIR"

# Download helper with local caching
get_cached_asset() {
    local url="$1"
    local subdir="$2"
    local filename="$3"
    local dest_dir="${CACHE_DIR}/${subdir}"
    local dest_file="${dest_dir}/${filename}"
    local local_fallback="bin/${filename}"

    mkdir -p "$dest_dir"

    if [[ -f "$dest_file" && -s "$dest_file" ]]; then
        echo "$dest_file"
        return 0
    fi

    # Check project's local bin folder as fallback
    if [[ -f "$local_fallback" && -s "$local_fallback" ]]; then
        cp -f "$local_fallback" "$dest_file"
        echo "$dest_file"
        return 0
    fi

    echo -e "${GRAY}  -> Downloading: ${url}${NC}" >&2
    if curl -sSL -f -o "$dest_file" "$url" 2>/dev/null; then
        echo "$dest_file"
        return 0
    elif command -v wget >/dev/null 2>&1 && wget -q -O "$dest_file" "$url" 2>/dev/null; then
        echo "$dest_file"
        return 0
    else
        rm -f "$dest_file"
        if [[ -f "$local_fallback" && -s "$local_fallback" ]]; then
            echo -e "${YELLOW}  Warning: Download failed, using local bin fallback: ${local_fallback}${NC}" >&2
            cp -f "$local_fallback" "$dest_file"
            echo "$dest_file"
            return 0
        fi
        echo -e "${RED}Failed to download: ${url}${NC}" >&2
        return 1
    fi
}

# Clean output dir
if [[ $CLEAN -eq 1 ]]; then
    echo -e "${YELLOW}Cleaning output directory: ${OUTPUT_DIR}${NC}"
    if [[ -d "$OUTPUT_DIR" ]]; then
        TMP_CACHE=""
        if [[ -d "$CACHE_DIR" ]]; then
            TMP_CACHE=$(mktemp -d 2>/dev/null || mktemp -d -t 'txui-cache')
            mv "$CACHE_DIR" "$TMP_CACHE"
        fi
        rm -rf "$OUTPUT_DIR"
        mkdir -p "$OUTPUT_DIR"
        if [[ -n "$TMP_CACHE" && -d "$TMP_CACHE/.cache" ]]; then
            mv "$TMP_CACHE/.cache" "$OUTPUT_DIR/"
            rm -rf "$TMP_CACHE"
        fi
    fi
else
    mkdir -p "$OUTPUT_DIR"
fi

echo -e "${CYAN}==========================================================${NC}"
echo -e "${CYAN} TX-UI Cross Build Script${NC}"
echo -e "${GRAY} Host Platform : ${HOST_PLATFORM}${NC}"
echo -e "${GRAY} Targets       : ${UNIQUE_TARGETS[*]}${NC}"
echo -e "${GRAY} Package Mode  : $(if [[ $PACKAGE -eq 1 ]]; then echo "Yes (Full release archive)"; else echo "No (Binary only)"; fi)${NC}"
echo -e "${GRAY} Output Dir    : ${OUTPUT_DIR}${NC}"
echo -e "${CYAN}==========================================================${NC}"

SUCCESS_COUNT=0
FAIL_COUNT=0

for target in "${UNIQUE_TARGETS[@]}"; do
    info=$(get_target_info "$target")
    canonical=$(echo "$info" | cut -d'|' -f1)
    t_goos=$(echo "$info" | cut -d'|' -f2)
    t_goarch=$(echo "$info" | cut -d'|' -f3)
    t_goarm=$(echo "$info" | cut -d'|' -f4)
    t_xray_zip=$(echo "$info" | cut -d'|' -f5)
    t_xray_bin=$(echo "$info" | cut -d'|' -f6)
    t_mtg_file=$(echo "$info" | cut -d'|' -f7)
    t_mtg_bin=$(echo "$info" | cut -d'|' -f8)
    t_zig_target=$(echo "$info" | cut -d'|' -f9)
    t_gcc_prefix=$(echo "$info" | cut -d'|' -f10)
    t_out_bin=$(echo "$info" | cut -d'|' -f11)
    t_archive=$(echo "$info" | cut -d'|' -f12)
    t_format=$(echo "$info" | cut -d'|' -f13)
    t_gomips=$(echo "$info" | cut -d'|' -f14)

    echo -e "\n${GREEN}>>> Building target: ${canonical} (${t_goos}/${t_goarch})${NC}"

    # Determine C compiler and CGO setting
    CGO_ENABLED="0"
    CC_BIN=""
    COMPILER_NOTE="Pure Go (CGO_ENABLED=0 with pure-Go SQLite)"

    if [[ "$CGO_MODE" != "0" ]]; then
        if [[ -n "${CC:-}" ]]; then
            CGO_ENABLED="1"
            CC_BIN="$CC"
            COMPILER_NOTE="Using CC from environment: $CC"
        elif [[ "$canonical" == "$HOST_PLATFORM" ]]; then
            if command -v gcc >/dev/null 2>&1; then
                CGO_ENABLED="1"
                CC_BIN="gcc"
                COMPILER_NOTE="Host GCC"
            elif command -v clang >/dev/null 2>&1; then
                CGO_ENABLED="1"
                CC_BIN="clang"
                COMPILER_NOTE="Host Clang"
            fi
        elif [[ -n "$t_gcc_prefix" ]] && command -v "${t_gcc_prefix}gcc" >/dev/null 2>&1; then
            CGO_ENABLED="1"
            CC_BIN="${t_gcc_prefix}gcc"
            COMPILER_NOTE="Target GCC: ${t_gcc_prefix}gcc"
        elif command -v zig >/dev/null 2>&1 && [[ -n "$t_zig_target" ]]; then
            CGO_ENABLED="1"
            CC_BIN="zig cc -target ${t_zig_target}"
            COMPILER_NOTE="Zig CC (${t_zig_target})"
        elif [[ "$CGO_MODE" == "1" ]]; then
            echo -e "${RED}Error: CGO is required (--cgo 1) for $canonical but no cross compiler was found.${NC}" >&2
            FAIL_COUNT=$((FAIL_COUNT + 1))
            continue
        fi
    fi

    echo -e "${GRAY}  Compiler  : ${COMPILER_NOTE}${NC}"

    # Staging paths
    target_dir="${OUTPUT_DIR}/${canonical}"
    staging_dir="${target_dir}/x-ui"
    bin_dir="${staging_dir}/bin"
    mkdir -p "$bin_dir"

    out_file="${staging_dir}/${t_out_bin}"
    direct_file="${OUTPUT_DIR}/x-ui-${canonical}$(if [[ "$t_goos" == "windows" ]]; then echo ".exe"; fi)"

    # Build Go binary
    build_cmd=("go" "build" "-ldflags" "$LDFLAGS")
    if [[ -n "$TAGS" ]]; then
        build_cmd+=("-tags" "$TAGS")
    fi
    build_cmd+=("-o" "$out_file" "main.go")

    echo -e "${CYAN}  -> Running: ${build_cmd[*]}${NC}"
    export GOOS="$t_goos"
    export GOARCH="$t_goarch"
    if [[ -n "$t_goarm" ]]; then export GOARM="$t_goarm"; else unset GOARM || true; fi
    if [[ -n "$t_gomips" ]]; then export GOMIPS="$t_gomips"; else unset GOMIPS || true; fi
    export CGO_ENABLED="$CGO_ENABLED"
    if [[ -n "$CC_BIN" ]]; then export CC="$CC_BIN"; else unset CC || true; fi

    if "${build_cmd[@]}"; then
        echo -e "${GREEN}  [OK] Binary built: ${out_file}${NC}"
        cp -f "$out_file" "$direct_file"
    else
        echo -e "${RED}  [FAIL] Compilation failed for ${canonical}${NC}" >&2
        FAIL_COUNT=$((FAIL_COUNT + 1))
        continue
    fi

    # Packaging & Asset Bundling
    if [[ $PACKAGE -eq 1 || $DOWNLOAD_ASSETS -eq 1 ]]; then
        echo -e "${CYAN}  -> Fetching runtime dependencies...${NC}"

        # 1. Xray-core
        if [[ -n "$t_xray_zip" ]]; then
            xray_url="https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/${t_xray_zip}"
            if cached_xray=$(get_cached_asset "$xray_url" "xray/${XRAY_VERSION}" "$t_xray_zip"); then
                tmp_extract="${target_dir}/_xray_tmp"
                mkdir -p "$tmp_extract"
                unzip -q -o "$cached_xray" -d "$tmp_extract"
                src_xray="${tmp_extract}/xray"
                if [[ "$t_goos" == "windows" ]]; then src_xray="${tmp_extract}/xray.exe"; fi
                if [[ -f "$src_xray" ]]; then
                    cp -f "$src_xray" "${bin_dir}/${t_xray_bin}"
                    chmod +x "${bin_dir}/${t_xray_bin}" 2>/dev/null || true
                    echo -e "${GREEN}  [OK] Xray-core (${XRAY_VERSION}) staged${NC}"
                fi
                rm -rf "$tmp_extract"
            fi
        fi

        # 2. mtg-multi
        if [[ -n "$t_mtg_file" && -n "$t_mtg_bin" ]]; then
            mtg_url="https://github.com/mhsanaei/mtg-multi/releases/download/v${MTG_VERSION}/${t_mtg_file}"
            if cached_mtg=$(get_cached_asset "$mtg_url" "mtg/${MTG_VERSION}" "$t_mtg_file"); then
                tmp_mtg="${target_dir}/_mtg_tmp"
                mkdir -p "$tmp_mtg"
                if [[ "$t_mtg_file" == *.zip ]]; then
                    unzip -q -o "$cached_mtg" -d "$tmp_mtg"
                else
                    tar -xzf "$cached_mtg" -C "$tmp_mtg" 2>/dev/null || true
                fi
                src_mtg=$(find "$tmp_mtg" -type f \( -name "mtg-multi" -o -name "mtg-multi.exe" -o -name "mtg" \) | head -n1)
                if [[ -n "$src_mtg" && -f "$src_mtg" ]]; then
                    cp -f "$src_mtg" "${bin_dir}/${t_mtg_bin}"
                    chmod +x "${bin_dir}/${t_mtg_bin}" 2>/dev/null || true
                    echo -e "${GREEN}  [OK] mtg-multi (v${MTG_VERSION}) staged${NC}"
                fi
                rm -rf "$tmp_mtg"
            fi
        fi

        # 3. Geodata files
        declare -A geo_map=(
            ["geoip.dat"]="https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
            ["geosite.dat"]="https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
            ["geoip_IR.dat"]="https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat"
            ["geosite_IR.dat"]="https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat"
            ["geoip_RU.dat"]="https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat"
            ["geosite_RU.dat"]="https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat"
        )
        for gname in "${!geo_map[@]}"; do
            gurl="${geo_map[$gname]}"
            if cgeo=$(get_cached_asset "$gurl" "geo" "$gname"); then
                cp -f "$cgeo" "${bin_dir}/${gname}"
            fi
        done
        ver_tag="latest-$(date +%Y%m%d)"
        echo -n "$ver_tag" > "${bin_dir}/geoip.dat.version"
        echo -n "$ver_tag" > "${bin_dir}/geosite.dat.version"
        echo -e "${GREEN}  [OK] Geodata rules staged${NC}"

        # 4. Service units and scripts
        [[ -f "x-ui.sh" ]] && cp -f "x-ui.sh" "$staging_dir/"
        [[ -f "x-ui.service" ]] && cp -f "x-ui.service" "$staging_dir/"
        if [[ "$t_goos" == "windows" ]]; then
            [[ -f "bin/wintun.dll" ]] && cp -f "bin/wintun.dll" "$bin_dir/"
            [[ -f "bin/xray_no_window.ps1" ]] && cp -f "bin/xray_no_window.ps1" "$bin_dir/"
            [[ -f "bin/xray_no_window.vbs" ]] && cp -f "bin/xray_no_window.vbs" "$bin_dir/"
        fi

        # 5. Archive package
        if [[ $PACKAGE -eq 1 ]]; then
            archive_path="${OUTPUT_DIR}/${t_archive}"
            rm -f "$archive_path"
            echo -e "${CYAN}  -> Packaging archive: ${archive_path}${NC}"

            if [[ "$t_format" == "zip" ]]; then
                if command -v zip >/dev/null 2>&1; then
                    (cd "$target_dir" && zip -q -r "$OLDPWD/$archive_path" "x-ui")
                elif command -v 7z >/dev/null 2>&1; then
                    7z a -tzip "$archive_path" "${staging_dir}/*" >/dev/null
                fi
            else
                if command -v tar >/dev/null 2>&1; then
                    tar -czf "$archive_path" -C "$target_dir" "x-ui"
                elif command -v 7z >/dev/null 2>&1; then
                    tar_tmp="${archive_path%.gz}"
                    7z a -ttar "$tar_tmp" "${staging_dir}/*" >/dev/null
                    7z a -tgzip "$archive_path" "$tar_tmp" >/dev/null
                    rm -f "$tar_tmp"
                fi
            fi

            if [[ -f "$archive_path" ]]; then
                echo -e "${GREEN}  [OK] Package created: $(basename "$archive_path")${NC}"
            fi
        fi
    fi

    SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
done

echo -e "\n${CYAN}==========================================================${NC}"
echo -e "${GREEN} Build Complete! Succeeded: ${SUCCESS_COUNT}, Failed: ${FAIL_COUNT}${NC}"
echo -e "${GRAY} Artifacts location: ${OUTPUT_DIR}${NC}"
echo -e "${CYAN}==========================================================${NC}\n"

if [[ $FAIL_COUNT -gt 0 ]]; then
    exit 1
fi
