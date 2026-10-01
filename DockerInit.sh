#!/bin/sh
set -eu

case "${1:-}" in
    amd64)
        ARCH="64"
        FNAME="amd64"
        ;;
    i386 | 386)
        ARCH="32"
        FNAME="386"
        ;;
    armv8 | arm64 | aarch64)
        ARCH="arm64-v8a"
        FNAME="arm64"
        ;;
    armv7 | arm32)
        ARCH="arm32-v7a"
        FNAME="arm"
        ;;
    armv6)
        ARCH="arm32-v6"
        FNAME="arm"
        ;;
    arm)
        case "${2:-}" in
            v6)
                ARCH="arm32-v6"
                FNAME="arm"
                ;;
            v7)
                ARCH="arm32-v7a"
                FNAME="arm"
                ;;
            *)
                echo "Unsupported ARM TARGETVARIANT: ${2:-}" >&2
                exit 1
                ;;
        esac
        ;;
    *)
        echo "Unsupported TARGETARCH/TARGETVARIANT: ${1:-}/${2:-}" >&2
        exit 1
        ;;
esac
mkdir -p build/bin
cd build/bin
wget -q "https://github.com/XTLS/Xray-core/releases/download/v26.9.30/Xray-linux-${ARCH}.zip"
unzip "Xray-linux-${ARCH}.zip"
rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat
mv xray "xray-linux-${FNAME}"
wget -q https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
wget -q https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
wget -q -O geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
wget -q -O geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
wget -q -O geoip_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat
wget -q -O geosite_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat
cd ../../
