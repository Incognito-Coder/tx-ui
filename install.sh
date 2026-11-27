#!/bin/sh

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# Basic logging helpers (POSIX sh / ash / bash compatible)
LOGD() {
    printf '%b\n' "${yellow}[DEG] $* ${plain}"
}

LOGE() {
    printf '%b\n' "${red}[ERR] $* ${plain}"
}

LOGI() {
    printf '%b\n' "${green}[INF] $* ${plain}"
}

# Portable interactive prompt helper (works even when script is piped to sh)
prompt_read() {
    _prompt_text="$1"
    _var_name="$2"
    _default_val="$3"
    _ans=""
    printf '%s' "$_prompt_text"
    if [ -t 0 ]; then
        read -r _ans || _ans=""
    elif [ -r /dev/tty ]; then
        read -r _ans < /dev/tty || _ans=""
    fi
    if [ -z "$_ans" ] && [ -n "$_default_val" ]; then
        _ans="$_default_val"
    fi
    eval "$_var_name=\$_ans"
}

# Check root privilege (POSIX id -u works in ash/dash/bash)
if [ "$(id -u)" -ne 0 ]; then
    printf '%b\n' "${red}Fatal error: ${plain} Please run this script with root privilege\n"
    exit 1
fi

# Check OS and set release variable
release=""
if [ -f /etc/os-release ]; then
    . /etc/os-release
    release=${ID:-}
elif [ -f /usr/lib/os-release ]; then
    . /usr/lib/os-release
    release=${ID:-}
fi

if [ -f /etc/openwrt_release ] || [ "${release}" = "openwrt" ] || [ "${release}" = "immortalwrt" ] || [ "${release}" = "lede" ] || grep -qiE "openwrt|immortalwrt|lede" /etc/os-release /usr/lib/os-release 2>/dev/null; then
    release="openwrt"
elif [ -z "${release}" ]; then
    echo "Failed to check the system OS, please contact the author!" >&2
    exit 1
fi
echo "The OS release is: $release"

detect_libc() {
    if [ -f /etc/openwrt_release ] || ls /lib/ld-musl-*.so* >/dev/null 2>&1; then
        echo "musl"
        return 0
    fi
    if command -v ldd >/dev/null 2>&1 && ldd --version 2>&1 | grep -qi "musl"; then
        echo "musl"
        return 0
    fi
    echo "glibc"
}

detect_init_system() {
    if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system -o -d /etc/systemd/system ]; then
        echo "systemd"
    elif [ "$release" = "openwrt" ] || [ -f /etc/rc.common ]; then
        echo "procd"
    elif command -v rc-service >/dev/null 2>&1; then
        echo "openrc"
    else
        echo "unknown"
    fi
}

# Inspect ELF byte 5 (EI_DATA) to distinguish big-endian vs little-endian MIPS kernels
detect_endianness() {
    if [ -f /etc/openwrt_release ]; then
        ow_arch=$(grep -E '^DISTRIB_ARCH=' /etc/openwrt_release 2>/dev/null | cut -d'=' -f2 | tr -d "'\"")
        case "$ow_arch" in
            mipsel*|mips64el*) echo "le"; return 0 ;;
            mips*|mips64*) echo "be"; return 0 ;;
        esac
    fi
    if command -v opkg >/dev/null 2>&1; then
        opkg_arch=$(opkg print-architecture 2>/dev/null | awk '{print $2}' | tr '\n' ' ')
        case "$opkg_arch" in
            *mipsel*|*mips64el*) echo "le"; return 0 ;;
            *mips*) echo "be"; return 0 ;;
        esac
    fi
    _elf_data=""
    if command -v hexdump >/dev/null 2>&1; then
        _elf_data=$(hexdump -n 1 -s 5 -e '1/1 "%d"' /bin/sh 2>/dev/null)
    elif command -v od >/dev/null 2>&1; then
        _elf_data=$(od -An -td1 -j5 -N1 /bin/sh 2>/dev/null | tr -d ' \n')
    fi
    if [ "$_elf_data" = "1" ]; then
        echo "le"
    else
        echo "be"
    fi
}

arch() {
    _uarch=$(uname -m)
    case "$_uarch" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64*) echo 'arm64' ;;
        armv7* | armv7) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        arm)
            _ow_arm=""
            if [ -f /etc/openwrt_release ]; then
                _ow_arm=$(grep -E '^DISTRIB_ARCH=' /etc/openwrt_release 2>/dev/null | cut -d'=' -f2 | tr -d "'\"")
            fi
            case "$_ow_arm" in
                *cortex-a*|*armv7*|arm_mpcore*|arm_xscale*) echo 'armv7' ;;
                *arm1176*) echo 'armv6' ;;
                *arm926*) echo 'armv5' ;;
                *)
                    if grep -qiE 'ARMv7|v7l' /proc/cpuinfo 2>/dev/null; then
                        echo 'armv7'
                    elif grep -qiE 'ARMv6|v6l' /proc/cpuinfo 2>/dev/null; then
                        echo 'armv6'
                    elif grep -qiE 'ARMv5|v5tel' /proc/cpuinfo 2>/dev/null; then
                        echo 'armv5'
                    else
                        echo 'armv7'
                    fi
                    ;;
            esac
            ;;
        s390x) echo 's390x' ;;
        riscv64) echo 'riscv64' ;;
        mips)
            # Linux kernel uname -m returns "mips" for both BE and LE 32-bit MIPS CPUs (e.g. MT7621).
            if [ "$(detect_endianness)" = "le" ]; then
                echo 'mipsle'
            else
                echo 'mips'
            fi
            ;;
        mipsle | mipsel) echo 'mipsle' ;;
        mips64)
            if [ "$(detect_endianness)" = "le" ]; then
                echo 'mips64le'
            else
                echo 'mips64'
            fi
            ;;
        mips64le | mips64el) echo 'mips64le' ;;
        ppc64le) echo 'ppc64le' ;;
        loong64 | loongarch64) echo 'loong64' ;;
        *)
            printf '%b\n' "${red}Unsupported CPU architecture: $_uarch ${plain}" >&2
            rm -f install.sh
            exit 1
            ;;
    esac
}

echo "arch: $(arch)"

version_ge() {
    awk -v cur="$1" -v req="$2" 'BEGIN {
        n1 = split(cur, a, ".");
        n2 = split(req, b, ".");
        n = (n1 > n2 ? n1 : n2);
        for (i = 1; i <= n; i++) {
            x = (i in a ? a[i]+0 : 0);
            y = (i in b ? b[i]+0 : 0);
            if (x > y) exit 0;
            if (x < y) exit 1;
        }
        exit 0;
    }'
}

check_libc() {
    libc_type=$(detect_libc)
    if [ "$libc_type" = "musl" ]; then
        echo "C library: musl libc (OpenWrt / Alpine compatible)"
        return 0
    fi

    if command -v ldd >/dev/null 2>&1; then
        glibc_version=$(ldd --version 2>/dev/null | head -n 1 | awk '{print $NF}')
        required_version="2.32"
        if [ -n "$glibc_version" ]; then
            if ! version_ge "$glibc_version" "$required_version"; then
                printf '%b\n' "${yellow}GLIBC version $glibc_version (< $required_version) detected. Static musl/pure-Go releases remain compatible.${plain}"
            else
                echo "GLIBC version: $glibc_version (meets requirement of 2.32+)"
            fi
        fi
    fi
}
check_libc

download_file() {
    _url="$1"
    _dest="$2"
    if command -v curl >/dev/null 2>&1; then
        if curl -fsSL -o "$_dest" "$_url" 2>/dev/null; then
            return 0
        fi
        if curl -fsSLk -o "$_dest" "$_url" 2>/dev/null; then
            return 0
        fi
    fi
    if command -v wget >/dev/null 2>&1; then
        if wget -q -O "$_dest" "$_url" 2>/dev/null; then
            return 0
        fi
        if wget -q --no-check-certificate -O "$_dest" "$_url" 2>/dev/null; then
            return 0
        fi
        if wget -N --no-check-certificate -O "$_dest" "$_url" 2>/dev/null; then
            return 0
        fi
    fi
    return 1
}

fetch_text() {
    _url="$1"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$_url" 2>/dev/null || curl -fsSLk "$_url" 2>/dev/null
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O - "$_url" 2>/dev/null || wget -q --no-check-certificate -O - "$_url" 2>/dev/null
    fi
}

install_base() {
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q wget curl tar tzdata socat openssl
            ;;
        centos | rhel | almalinux | rocky | ol)
            yum -y update && yum install -y -q wget curl tar tzdata socat openssl
            ;;
        fedora | amzn)
            dnf -y update && dnf install -y -q wget curl tar tzdata socat openssl
            ;;
        arch | manjaro | parch)
            pacman -Syu && pacman -Syu --noconfirm wget curl tar tzdata socat openssl
            ;;
        opensuse-tumbleweed | opensuse)
            zypper refresh && zypper -q install -y wget curl tar timezone socat openssl
            ;;
        alpine)
            apk update && apk add wget curl tar tzdata socat openssl bash
            ;;
        openwrt)
            if command -v opkg >/dev/null 2>&1; then
                opkg update || true
                opkg install curl ca-bundle ca-certificates tar gzip socat openssl-util || true
                command -v curl >/dev/null 2>&1 || opkg install curl ca-bundle || opkg install wget-ssl ca-bundle || true
                command -v openssl >/dev/null 2>&1 || opkg install openssl-util || opkg install openssl || true
            elif command -v apk >/dev/null 2>&1; then
                apk update || true
                apk add curl ca-bundle ca-certificates tar gzip socat openssl-util || apk add curl ca-certificates tar socat openssl || true
            fi
            ;;
        gentoo)
            emerge --sync && emerge --ask --quiet wget curl tar tzdata socat openssl
            ;;
        clearlinux)
            swupd update && swupd bundle-add wget curl tar tzdata socat openssl
            ;;
        void)
            xbps-install -S && xbps-install -y wget curl tar tzdata socat openssl
            ;;
        solus)
            eopkg update && eopkg install -y wget curl tar tzdata socat openssl
            ;;
        *)
            if command -v apt-get >/dev/null 2>&1; then
                apt-get update && apt-get install -y -q wget curl tar tzdata socat openssl
            elif command -v opkg >/dev/null 2>&1; then
                opkg update && opkg install curl ca-bundle tar socat openssl-util || true
            elif command -v apk >/dev/null 2>&1; then
                apk update && apk add curl ca-certificates tar socat openssl || true
            fi
            ;;
    esac
}

gen_random_string() {
    _length="$1"
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 $(( _length * 2 )) 2>/dev/null \
            | tr -dc 'a-zA-Z0-9' \
            | head -c "$_length"
    else
        tr -dc 'a-zA-Z0-9' < /dev/urandom 2>/dev/null | head -c "$_length"
    fi
}

gen_random_port() {
    if command -v shuf >/dev/null 2>&1; then
        shuf -i 1024-62000 -n 1
    elif command -v od >/dev/null 2>&1; then
        od -An -N2 -tu2 /dev/urandom 2>/dev/null | awk '{print ($1 % 60000) + 1024}'
    else
        awk 'BEGIN{srand(); print int(1024 + rand() * 60000)}'
    fi
}

get_server_ip() {
    _ip=$(fetch_text "https://api.ipify.org" | tr -d ' \r\n')
    if [ -z "$_ip" ] && command -v ip >/dev/null 2>&1; then
        _ip=$(ip -4 addr show 2>/dev/null | awk '/inet / && !/127\.0\.0\.1/ {sub(/\/.*/, "", $2); print $2; exit}')
    fi
    echo "${_ip:-127.0.0.1}"
}

generate_self_signed_cert() {
    _host="$1"
    _cert_dir="/root/cert/${_host}"
    mkdir -p "$_cert_dir"
    if ! command -v openssl >/dev/null 2>&1; then
        LOGE "openssl not found; skipping self-signed certificate generation."
        return 1
    fi
    openssl req -x509 -newkey rsa:4096 \
        -keyout "${_cert_dir}/privkey.pem" \
        -out "${_cert_dir}/fullchain.pem" \
        -days 365 -nodes -subj "/CN=${_host}"
    if [ $? -ne 0 ]; then
        LOGE "Generating self-signed certificate failed."
        return 1
    fi
    LOGI "Generating self-signed certificate succeeded."
    return 0
}

svc_stop() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd) systemctl stop x-ui 2>/dev/null || true ;;
        procd)
            if [ -x /etc/init.d/x-ui ]; then
                /etc/init.d/x-ui stop 2>/dev/null || true
            fi
            ;;
        openrc) rc-service x-ui stop 2>/dev/null || true ;;
    esac
}

svc_enable_and_start() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd)
            mkdir -p /etc/systemd/system
            cp -f /usr/local/x-ui/x-ui.service /etc/systemd/system/
            systemctl daemon-reload
            systemctl enable x-ui
            systemctl start x-ui
            ;;
        procd)
            if [ -f /usr/local/x-ui/x-ui.init ]; then
                cp -f /usr/local/x-ui/x-ui.init /etc/init.d/x-ui
            else
                download_file "https://raw.githubusercontent.com/Incognito-Coder/tx-ui/main/x-ui.init" /etc/init.d/x-ui || true
            fi
            chmod +x /etc/init.d/x-ui
            /etc/init.d/x-ui enable
            /etc/init.d/x-ui start
            ;;
        openrc)
            if [ -f /usr/local/x-ui/x-ui.init ]; then
                cp -f /usr/local/x-ui/x-ui.init /etc/init.d/x-ui
                chmod +x /etc/init.d/x-ui
                rc-update add x-ui default 2>/dev/null || true
                rc-service x-ui start 2>/dev/null || true
            fi
            ;;
    esac
}

config_after_install() {
    existing_hasDefaultCredential=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')

    if [ "${#existing_webBasePath}" -lt 4 ]; then
        if [ "$existing_hasDefaultCredential" = "true" ]; then
            config_webBasePath=$(gen_random_string 15)
            config_username=$(gen_random_string 10)
            config_password=$(gen_random_string 10)
            access_url=""

            if ! command -v openssl >/dev/null 2>&1 && [ ! -t 0 ] && [ ! -r /dev/tty ]; then
                choice="0"
            else
                printf '%b\n' "${yellow}Choose an option for SSL certificate:${plain}"
                printf '%b\n' "  1. Generate a self-signed certificate"
                printf '%b\n' "  2. Get a certificate from a domain name using acme.sh"
                printf '%b\n' "  3. Get a certificate for an IP address using acme.sh"
                printf '%b\n' "  4. Skip SSL setup (HTTP access)"
                prompt_read "Enter your choice [1-4, default 1]: " choice "1"
            fi

            case "$choice" in
                1)
                    server_ip=$(get_server_ip)
                    LOGI "Using IP address: ${server_ip}"
                    LOGD "Generating self-signed certificate for IP: ${server_ip}..."
                    if generate_self_signed_cert "${server_ip}"; then
                        webCertFile="/root/cert/${server_ip}/fullchain.pem"
                        webKeyFile="/root/cert/${server_ip}/privkey.pem"
                        if [ -f "$webCertFile" ] && [ -f "$webKeyFile" ]; then
                            /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                            LOGI "Panel paths set for IP: $server_ip"
                            LOGI "  - Certificate File: $webCertFile"
                            LOGI "  - Private Key File: $webKeyFile"
                            access_url="https://${server_ip}"
                        fi
                    fi
                    if [ -z "$access_url" ]; then
                        access_url="http://${server_ip}"
                    fi
                    ;;
                2)
                    if ! command -v ~/.acme.sh/acme.sh >/dev/null 2>&1; then
                        echo "acme.sh could not be found. Installing it..."
                        LOGI "Installing acme.sh..."
                        cd ~ || return 1
                        fetch_text "https://get.acme.sh" | sh
                        if [ $? -ne 0 ]; then
                            LOGE "Installation of acme.sh failed."
                        else
                            LOGI "Installation of acme.sh succeeded."
                        fi
                    fi

                    prompt_read "Enter your domain name: " domain ""
                    LOGI "Using domain: ${domain}"
                    LOGD "Your domain is: ${domain}, trying to issue a certificate..."

                    certPath="/root/cert/${domain}"
                    rm -rf "$certPath"
                    mkdir -p "$certPath"

                    if command -v ~/.acme.sh/acme.sh >/dev/null 2>&1; then
                        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
                        ~/.acme.sh/acme.sh --issue -d "${domain}" --listen-v6 --standalone --httpport 80
                        if [ $? -ne 0 ]; then
                            LOGE "Issuing certificate with acme.sh failed, falling back to self-signed certificate."
                            rm -rf ~/.acme.sh/"${domain}"
                            generate_self_signed_cert "${domain}"
                        else
                            LOGI "Issuing certificate succeeded, installing certificates..."
                            ~/.acme.sh/acme.sh --installcert -d "${domain}" \
                                --key-file "/root/cert/${domain}/privkey.pem" \
                                --fullchain-file "/root/cert/${domain}/fullchain.pem"
                            if [ $? -ne 0 ]; then
                                LOGE "Installing certificate failed."
                                rm -rf ~/.acme.sh/"${domain}"
                            else
                                LOGI "Installing certificate succeeded, enabling auto renew..."
                                ~/.acme.sh/acme.sh --upgrade --auto-upgrade
                            fi
                        fi
                    else
                        LOGE "acme.sh is not installed, falling back to self-signed certificate."
                        generate_self_signed_cert "${domain}"
                    fi

                    webCertFile="/root/cert/${domain}/fullchain.pem"
                    webKeyFile="/root/cert/${domain}/privkey.pem"
                    if [ -f "$webCertFile" ] && [ -f "$webKeyFile" ]; then
                        /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                        LOGI "Panel paths set for domain: $domain"
                        access_url="https://${domain}"
                    else
                        access_url="http://${domain}"
                    fi
                    ;;
                3)
                    if ! command -v ~/.acme.sh/acme.sh >/dev/null 2>&1; then
                        echo "acme.sh could not be found. Installing it..."
                        LOGI "Installing acme.sh..."
                        cd ~ || return 1
                        fetch_text "https://get.acme.sh" | sh
                    fi

                    server_ip=$(get_server_ip)
                    LOGI "Using IP address: ${server_ip}"
                    LOGD "Your IP is: ${server_ip}, trying to issue a certificate..."

                    certPath="/root/cert/${server_ip}"
                    rm -rf "$certPath"
                    mkdir -p "$certPath"

                    if command -v ~/.acme.sh/acme.sh >/dev/null 2>&1; then
                        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
                        ~/.acme.sh/acme.sh --issue -d "${server_ip}" --standalone --server letsencrypt --certificate-profile shortlived --days 6 --httpport 80 --force
                        if [ $? -ne 0 ]; then
                            LOGE "Issuing certificate with acme.sh failed, falling back to self-signed certificate."
                            rm -rf ~/.acme.sh/"${server_ip}"
                            generate_self_signed_cert "${server_ip}"
                        else
                            LOGI "Issuing certificate succeeded, installing certificates..."
                            ~/.acme.sh/acme.sh --installcert -d "${server_ip}" \
                                --key-file "/root/cert/${server_ip}/privkey.pem" \
                                --fullchain-file "/root/cert/${server_ip}/fullchain.pem"
                            if [ $? -ne 0 ]; then
                                LOGE "Installing certificate failed."
                                rm -rf ~/.acme.sh/"${server_ip}"
                            else
                                LOGI "Installing certificate succeeded, enabling auto renew..."
                                ~/.acme.sh/acme.sh --upgrade --auto-upgrade
                            fi
                        fi
                    else
                        LOGE "acme.sh is not installed, falling back to self-signed certificate."
                        generate_self_signed_cert "${server_ip}"
                    fi

                    webCertFile="/root/cert/${server_ip}/fullchain.pem"
                    webKeyFile="/root/cert/${server_ip}/privkey.pem"
                    if [ -f "$webCertFile" ] && [ -f "$webKeyFile" ]; then
                        /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                        access_url="https://${server_ip}"
                    else
                        access_url="http://${server_ip}"
                    fi
                    ;;
                4 | 0)
                    server_ip=$(get_server_ip)
                    access_url="http://${server_ip}"
                    LOGI "Skipping SSL certificate setup. You can configure SSL later from the panel or x-ui menu."
                    ;;
                *)
                    echo "Invalid choice. Exiting."
                    exit 1
                    ;;
            esac

            config_port=$(gen_random_port)
            printf '%b\n' "${yellow}Generated random port: ${config_port}${plain}"

            /usr/local/x-ui/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"
            printf '%b\n' "This is a fresh installation, generating random login info for security concerns:"
            printf '%b\n' "###############################################"
            printf '%b\n' "${green}Username: ${config_username}${plain}"
            printf '%b\n' "${green}Password: ${config_password}${plain}"
            printf '%b\n' "${green}Port: ${config_port}${plain}"
            printf '%b\n' "${green}WebBasePath: ${config_webBasePath}${plain}"
            printf '%b\n' "${green}Access URL: ${access_url}:${config_port}/${config_webBasePath}${plain}"
            printf '%b\n' "###############################################"
        else
            config_webBasePath=$(gen_random_string 15)
            printf '%b\n' "${yellow}WebBasePath is missing or too short. Generating a new one...${plain}"
            /usr/local/x-ui/x-ui setting -webBasePath "${config_webBasePath}"
            printf '%b\n' "${green}New WebBasePath: ${config_webBasePath}${plain}"
            server_ip=$(get_server_ip)
            printf '%b\n' "${green}Access URL: http://${server_ip}:${existing_port}/${config_webBasePath}${plain}"
        fi
    else
        if [ "$existing_hasDefaultCredential" = "true" ]; then
            config_username=$(gen_random_string 10)
            config_password=$(gen_random_string 10)

            printf '%b\n' "${yellow}Default credentials detected. Security update required...${plain}"
            /usr/local/x-ui/x-ui setting -username "${config_username}" -password "${config_password}"
            printf '%b\n' "Generated new random login credentials:"
            printf '%b\n' "###############################################"
            printf '%b\n' "${green}Username: ${config_username}${plain}"
            printf '%b\n' "${green}Password: ${config_password}${plain}"
            printf '%b\n' "###############################################"
        else
            printf '%b\n' "${green}Username, Password, and WebBasePath are properly set. Continuing...${plain}"
        fi
    fi

    /usr/local/x-ui/x-ui migrate
}

install_x_ui() {
    mkdir -p /usr/local /usr/bin
    cd /usr/local || exit 1

    target_arch=$(arch)

    # Warn on OpenWrt routers with small overlay storage
    if [ "$release" = "openwrt" ] && command -v df >/dev/null 2>&1; then
        free_mb=$(df -Pm /usr/local 2>/dev/null | awk 'NR==2 {print $4}')
        if [ -n "$free_mb" ] && [ "$free_mb" -lt 110 ] 2>/dev/null; then
            printf '%b\n' "${yellow}Warning: Only ${free_mb} MB free space detected on /usr/local.${plain}"
            printf '%b\n' "${yellow}Full archive with geodata requires ~100 MB. Consider USB extroot or removing unused geo*.dat files after install.${plain}"
        fi
    fi

    if [ $# -eq 0 ]; then
        tag_version=$(fetch_text "https://api.github.com/repos/Incognito-Coder/tx-ui/releases/latest" \
            | grep '"tag_name":' \
            | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' \
            | head -n 1)
        if [ -z "$tag_version" ]; then
            printf '%b\n' "${red}Failed to fetch x-ui version, it may be due to GitHub API restrictions, please try it later${plain}"
            exit 1
        fi
        printf '%b\n' "Got x-ui latest version: ${tag_version}, beginning the installation..."
        archive_url="https://github.com/Incognito-Coder/tx-ui/releases/download/${tag_version}/x-ui-linux-${target_arch}.tar.gz"
        if ! download_file "$archive_url" "/usr/local/x-ui-linux-${target_arch}.tar.gz"; then
            printf '%b\n' "${red}Downloading x-ui failed, please be sure that your server/router can access GitHub${plain}"
            exit 1
        fi
    else
        tag_version=$1
        case "$tag_version" in
            v*) ;;
            *) tag_version="v$tag_version" ;;
        esac
        tag_version_numeric=${tag_version#v}
        min_version="0.1"

        if ! version_ge "$tag_version_numeric" "$min_version"; then
            printf '%b\n' "${red}Please use a newer version (at least v0.1). Exiting installation.${plain}"
            exit 1
        fi

        archive_url="https://github.com/Incognito-Coder/tx-ui/releases/download/${tag_version}/x-ui-linux-${target_arch}.tar.gz"
        printf '%b\n' "Beginning to install tx-ui $1"
        if ! download_file "$archive_url" "/usr/local/x-ui-linux-${target_arch}.tar.gz"; then
            printf '%b\n' "${red}Download tx-ui $1 failed, please check if the version exists${plain}"
            exit 1
        fi
    fi

    if [ -e /usr/local/x-ui/ ]; then
        svc_stop
        mkdir -p /etc/x-ui
        cp -f /usr/local/x-ui/bin/*.version /etc/x-ui/ 2>/dev/null || true
        rm -rf /usr/local/x-ui/
    fi

    if ! tar zxf "/usr/local/x-ui-linux-${target_arch}.tar.gz" -C /usr/local/; then
        printf '%b\n' "${red}Failed to extract archive! Check disk space or downloaded file integrity.${plain}"
        rm -f "/usr/local/x-ui-linux-${target_arch}.tar.gz"
        exit 1
    fi
    rm -f "/usr/local/x-ui-linux-${target_arch}.tar.gz"
    cd /usr/local/x-ui || exit 1
    chmod +x x-ui

    # Restore geodata version files if missing after extraction
    if [ -d /etc/x-ui ]; then
        for f in /etc/x-ui/*.version; do
            if [ -f "$f" ] && [ ! -f "bin/$(basename "$f")" ]; then
                cp -f "$f" bin/ 2>/dev/null || true
            fi
        done
    fi

    # Check the system's architecture and rename ARM sub-binaries accordingly
    if [ "$target_arch" = "armv5" ] || [ "$target_arch" = "armv6" ] || [ "$target_arch" = "armv7" ]; then
        mv "bin/xray-linux-${target_arch}" bin/xray-linux-arm 2>/dev/null || true
        chmod +x bin/xray-linux-arm 2>/dev/null || true
        mv "bin/mtg-linux-${target_arch}" bin/mtg-linux-arm 2>/dev/null || true
        chmod +x bin/mtg-linux-arm 2>/dev/null || true
    fi

    if [ -f "bin/xray-linux-${target_arch}" ]; then
        chmod +x "bin/xray-linux-${target_arch}"
    fi
    if [ -f "bin/mtg-linux-${target_arch}" ]; then
        chmod +x "bin/mtg-linux-${target_arch}"
    fi
    chmod +x bin/mtg* 2>/dev/null || true

    # Install management CLI script
    if ! download_file "https://raw.githubusercontent.com/Incognito-Coder/tx-ui/main/x-ui.sh" /usr/bin/x-ui; then
        if [ -f /usr/local/x-ui/x-ui.sh ]; then
            cp -f /usr/local/x-ui/x-ui.sh /usr/bin/x-ui
        fi
    fi
    chmod +x /usr/local/x-ui/x-ui.sh 2>/dev/null || true
    chmod +x /usr/bin/x-ui
    ln -sf /usr/bin/x-ui /usr/bin/tx-ui

    config_after_install
    svc_enable_and_start

    printf '%b\n' "${green}tx-ui ${tag_version}${plain} installation finished, it is running now...\n"
    if [ "$release" = "openwrt" ]; then
        printf '%b\n' "${yellow}OpenWrt note: Service installed as /etc/init.d/x-ui (procd). Remember to allow the panel/inbound ports in OpenWrt Firewall (Network -> Firewall -> Traffic Rules, or 'x-ui' -> option 21).${plain}\n"
    fi

    printf '%b\n' "┌───────────────────────────────────────────────────────┐"
    printf '%b\n' "│  ${blue}tx-ui / x-ui control menu usages (subcommands):${plain}      │"
    printf '%b\n' "│                                                       │"
    printf '%b\n' "│  ${blue}tx-ui / x-ui${plain}      - Admin Management Script          │"
    printf '%b\n' "│  ${blue}x-ui start${plain}        - Start                            │"
    printf '%b\n' "│  ${blue}x-ui stop${plain}         - Stop                             │"
    printf '%b\n' "│  ${blue}x-ui restart${plain}      - Restart                          │"
    printf '%b\n' "│  ${blue}x-ui status${plain}       - Current Status                   │"
    printf '%b\n' "│  ${blue}x-ui settings${plain}     - Current Settings                 │"
    printf '%b\n' "│  ${blue}x-ui enable${plain}       - Enable Autostart on OS Startup   │"
    printf '%b\n' "│  ${blue}x-ui disable${plain}      - Disable Autostart on OS Startup  │"
    printf '%b\n' "│  ${blue}x-ui log${plain}          - Check logs                       │"
    printf '%b\n' "│  ${blue}x-ui banlog${plain}       - Check Fail2ban ban logs          │"
    printf '%b\n' "│  ${blue}x-ui update${plain}       - Update                           │"
    printf '%b\n' "│  ${blue}x-ui legacy${plain}       - Legacy version                   │"
    printf '%b\n' "│  ${blue}x-ui install${plain}      - Install                          │"
    printf '%b\n' "│  ${blue}x-ui uninstall${plain}    - Uninstall                        │"
    printf '%b\n' "└───────────────────────────────────────────────────────┘"
}

printf '%b\n' "${green}Running...${plain}"
install_base
install_x_ui "$@"
