#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# Add some basic function here
function LOGD() {
    echo -e "${yellow}[DEG] $* ${plain}"
}

function LOGE() {
    echo -e "${red}[ERR] $* ${plain}"
}

function LOGI() {
    echo -e "${green}[INF] $* ${plain}"
}

# check root
if [[ $EUID -ne 0 ]]; then
    echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n "
    exit 1
fi

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "Failed to check the system OS, please contact the author!" >&2
    exit 1
fi
echo "The OS release is: $release"

arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}Unsupported CPU architecture! ${plain}" && rm -f install.sh && exit 1 ;;
    esac
}

echo "arch: $(arch)"

check_glibc_version() {
    glibc_version=$(ldd --version | head -n1 | awk '{print $NF}')
    required_version="2.32"
    if [[ "$(printf '%s\n' "$required_version" "$glibc_version" | sort -V | head -n1)" != "$required_version" ]]; then
        echo -e "${red}GLIBC version $glibc_version is too old! Required: 2.32 or higher${plain}"
        echo "Please upgrade to a newer version of your operating system to get a higher GLIBC version."
        exit 1
    fi
    echo "GLIBC version: $glibc_version (meets requirement of 2.32+)"
}
check_glibc_version

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
            apk update && apk add wget curl tar tzdata socat openssl
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
            apt-get update && apt install -y -q wget curl tar tzdata socat openssl
            ;;
    esac
}

gen_random_string() {
    local length="$1"
    openssl rand -base64 $(( length * 2 )) \
            | tr -dc 'a-zA-Z0-9' \
            | head -c "$length"
}

config_after_install() {
    local existing_hasDefaultCredential=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    local existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    local existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath=$(gen_random_string 15)
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            echo -e "${yellow}Choose an option for SSL certificate:${plain}"
            echo -e "  1. Generate a self-signed certificate"
            echo -e "  2. Get a certificate from a domain name using acme.sh"
            echo -e "  3. Get a certificate for an IP address using acme.sh"
            read -p "Enter your choice [1-3]: " choice

            case $choice in
                1)
                    # get the ip here
                    local server_ip=$(curl -s https://api.ipify.org)
                    LOGI "Using IP address: ${server_ip}"

                    LOGD "Generating self-signed certificate for IP: ${server_ip}..."

                    # create a directory for the certificate
                    certPath="/root/cert/${server_ip}"
                    if [ ! -d "$certPath" ]; then
                        mkdir -p "$certPath"
                    else
                        rm -rf "$certPath"
                        mkdir -p "$certPath"
                    fi

                    # generate self-signed cert
                    openssl req -x509 -newkey rsa:4096 -keyout /root/cert/${server_ip}/privkey.pem -out /root/cert/${server_ip}/fullchain.pem -days 365 -nodes -subj "/CN=${server_ip}"
                    if [ $? -ne 0 ]; then
                        LOGE "Generating self-signed certificate failed."
                    else
                        LOGI "Generating self-signed certificate succeeded."
                    fi

                    # Set panel paths after successful certificate installation
                    local webCertFile="/root/cert/${server_ip}/fullchain.pem"
                    local webKeyFile="/root/cert/${server_ip}/privkey.pem"

                    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
                        /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                        LOGI "Panel paths set for IP: $server_ip"
                        LOGI "  - Certificate File: $webCertFile"
                        LOGI "  - Private Key File: $webKeyFile"
                    else
                        LOGE "Error: Certificate or private key file not found for IP: $server_ip."
                    fi
                    local access_url="https://${server_ip}"
                    ;;
                2)
                    # check for acme.sh first
                    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
                        echo "acme.sh could not be found. we will install it"
                        LOGI "Installing acme.sh..."
                        cd ~ || return 1 # Ensure you can change to the home directory
                        curl -s https://get.acme.sh | sh
                        if [ $? -ne 0 ]; then
                            LOGE "Installation of acme.sh failed."
                        else
                            LOGI "Installation of acme.sh succeeded."
                        fi
                    fi
                    
                    read -p "Enter your domain name: " domain
                    LOGI "Using domain: ${domain}"

                    LOGD "Your domain is: ${domain}, trying to issue a certificate..."

                    # create a directory for the certificate
                    certPath="/root/cert/${domain}"
                    if [ ! -d "$certPath" ]; then
                        mkdir -p "$certPath"
                    else
                        rm -rf "$certPath"
                        mkdir -p "$certPath"
                    fi

                    # issue the certificate
                    if command -v ~/.acme.sh/acme.sh &>/dev/null; then
                        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
                        ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport 80
                        if [ $? -ne 0 ]; then
                            LOGE "Issuing certificate with acme.sh failed, falling back to self-signed certificate."
                            rm -rf ~/.acme.sh/${domain}
                            
                            # generate self-signed cert
                            openssl req -x509 -newkey rsa:4096 -keyout /root/cert/${domain}/privkey.pem -out /root/cert/${domain}/fullchain.pem -days 365 -nodes -subj "/CN=${domain}"
                            if [ $? -ne 0 ]; then
                                LOGE "Generating self-signed certificate failed."
                            else
                                LOGI "Generating self-signed certificate succeeded."
                            fi
                        else
                            LOGI "Issuing certificate succeeded, installing certificates..."
                            # install the certificate
                            ~/.acme.sh/acme.sh --installcert -d ${domain} \
                                --key-file /root/cert/${domain}/privkey.pem \
                                --fullchain-file /root/cert/${domain}/fullchain.pem

                            if [ $? -ne 0 ]; then
                                LOGE "Installing certificate failed."
                                rm -rf ~/.acme.sh/${domain}
                            else
                                LOGI "Installing certificate succeeded, enabling auto renew..."
                                # enable auto-renew
                                ~/.acme.sh/acme.sh --upgrade --auto-upgrade
                            fi
                        fi
                    else
                        LOGE "acme.sh is not installed, falling back to self-signed certificate."
                        openssl req -x509 -newkey rsa:4096 -keyout /root/cert/${domain}/privkey.pem -out /root/cert/${domain}/fullchain.pem -days 365 -nodes -subj "/CN=${domain}"
                        if [ $? -ne 0 ]; then
                            LOGE "Generating self-signed certificate failed."
                        else
                            LOGI "Generating self-signed certificate succeeded."
                        fi
                    fi

                    # Set panel paths after successful certificate installation
                    local webCertFile="/root/cert/${domain}/fullchain.pem"
                    local webKeyFile="/root/cert/${domain}/privkey.pem"

                    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
                        /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                        LOGI "Panel paths set for domain: $domain"
                        LOGI "  - Certificate File: $webCertFile"
                        LOGI "  - Private Key File: $webKeyFile"
                    else
                        LOGE "Error: Certificate or private key file not found for domain: $domain."
                    fi
                    local access_url="https://${domain}"
                    ;;
                3)
                    # check for acme.sh first
                    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
                        echo "acme.sh could not be found. we will install it"
                        LOGI "Installing acme.sh..."
                        cd ~ || return 1 # Ensure you can change to the home directory
                        curl -s https://get.acme.sh | sh
                        if [ $? -ne 0 ]; then
                            LOGE "Installation of acme.sh failed."
                        else
                            LOGI "Installation of acme.sh succeeded."
                        fi
                    fi

                    # get the ip here
                    local server_ip=$(curl -s https://api.ipify.org)
                    LOGI "Using IP address: ${server_ip}"

                    LOGD "Your IP is: ${server_ip}, trying to issue a certificate..."

                    # create a directory for the certificate
                    certPath="/root/cert/${server_ip}"
                    if [ ! -d "$certPath" ]; then
                        mkdir -p "$certPath"
                    else
                        rm -rf "$certPath"
                        mkdir -p "$certPath"
                    fi

                    # issue the certificate
                    if command -v ~/.acme.sh/acme.sh &>/dev/null; then
                        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
                        ~/.acme.sh/acme.sh --issue -d ${server_ip} --standalone --server letsencrypt --certificate-profile shortlived --days 6 --httpport 80 --force
                        if [ $? -ne 0 ]; then
                            LOGE "Issuing certificate with acme.sh failed, falling back to self-signed certificate."
                            rm -rf ~/.acme.sh/${server_ip}

                            # generate self-signed cert
                            openssl req -x509 -newkey rsa:4096 -keyout /root/cert/${server_ip}/privkey.pem -out /root/cert/${server_ip}/fullchain.pem -days 365 -nodes -subj "/CN=${server_ip}"
                            if [ $? -ne 0 ]; then
                                LOGE "Generating self-signed certificate failed."
                            else
                                LOGI "Generating self-signed certificate succeeded."
                            fi
                        else
                            LOGI "Issuing certificate succeeded, installing certificates..."
                            # install the certificate
                            ~/.acme.sh/acme.sh --installcert -d ${server_ip} \
                                --key-file /root/cert/${server_ip}/privkey.pem \
                                --fullchain-file /root/cert/${server_ip}/fullchain.pem

                            if [ $? -ne 0 ]; then
                                LOGE "Installing certificate failed."
                                rm -rf ~/.acme.sh/${server_ip}
                            else
                                LOGI "Installing certificate succeeded, enabling auto renew..."
                                # enable auto-renew
                                ~/.acme_sh/acme.sh --upgrade --auto-upgrade
                            fi
                        fi
                    else
                        LOGE "acme.sh is not installed, falling back to self-signed certificate."
                        openssl req -x509 -newkey rsa:4096 -keyout /root/cert/${server_ip}/privkey.pem -out /root/cert/${server_ip}/fullchain.pem -days 365 -nodes -subj "/CN=${server_ip}"
                        if [ $? -ne 0 ]; then
                            LOGE "Generating self-signed certificate failed."
                        else
                            LOGI "Generating self-signed certificate succeeded."
                        fi
                    fi

                    # Set panel paths after successful certificate installation
                    local webCertFile="/root/cert/${server_ip}/fullchain.pem"
                    local webKeyFile="/root/cert/${server_ip}/privkey.pem"

                    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
                        /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                        LOGI "Panel paths set for IP: $server_ip"
                        LOGI "  - Certificate File: $webCertFile"
                        LOGI "  - Private Key File: $webKeyFile"
                    else
                        LOGE "Error: Certificate or private key file not found for IP: $server_ip."
                    fi
                    local access_url="https://${server_ip}"
                    ;;
                *)
                    echo "Invalid choice. Exiting."
                    exit 1
                    ;;
            esac

            local config_port
            config_port=$(shuf -i 1024-62000 -n 1)
            echo -e "${yellow}Generated random port: ${config_port}${plain}"

            /usr/local/x-ui/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"
            echo -e "This is a fresh installation, generating random login info for security concerns:"
            echo -e "###############################################"
            echo -e "${green}Username: ${config_username}${plain}"
            echo -e "${green}Password: ${config_password}${plain}"
            echo -e "${green}Port: ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "${green}Access URL: ${access_url}:${config_port}/${config_webBasePath}${plain}"
            echo -e "###############################################"
        else
            local config_webBasePath=$(gen_random_string 15)
            echo -e "${yellow}WebBasePath is missing or too short. Generating a new one...${plain}"
            /usr/local/x-ui/x-ui setting -webBasePath "${config_webBasePath}"
            echo -e "${green}New WebBasePath: ${config_webBasePath}${plain}"
            local server_ip=$(curl -s https://api.ipify.org)
            echo -e "${green}Access URL: http://${server_ip}:${existing_port}/${config_webBasePath}${plain}"
        fi
    else
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            echo -e "${yellow}Default credentials detected. Security update required...${plain}"
            /usr/local/x-ui/x-ui setting -username "${config_username}" -password "${config_password}"
            echo -e "Generated new random login credentials:"
            echo -e "###############################################"
            echo -e "${green}Username: ${config_username}${plain}"
            echo -e "${green}Password: ${config_password}${plain}"
            echo -e "###############################################"
        else
            echo -e "${green}Username, Password, and WebBasePath are properly set. Exiting...${plain}"
        fi
    fi

    /usr/local/x-ui/x-ui migrate
}

install_x-ui() {
    cd /usr/local/

    if [ $# == 0 ]; then
        tag_version=$(curl -Ls "https://api.github.com/repos/Incognito-Coder/tx-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${red}Failed to fetch x-ui version, it may be due to GitHub API restrictions, please try it later${plain}"
            exit 1
        fi
        echo -e "Got x-ui latest version: ${tag_version}, beginning the installation..."
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(arch).tar.gz https://github.com/Incognito-Coder/tx-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Downloading x-ui failed, please be sure that your server can access GitHub ${plain}"
            exit 1
        fi
    else
        tag_version=$1
        tag_version_numeric=${tag_version#v}
        min_version="0.1"

        if [[ "$(printf '%s\n' "$min_version" "$tag_version_numeric" | sort -V | head -n1)" != "$min_version" ]]; then
            echo -e "${red}Please use a newer version (at least v0.1). Exiting installation.${plain}"
            exit 1
        fi

        url="https://github.com/Incognito-Coder/tx-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz"
        echo -e "Beginning to install x-ui $1"
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Download x-ui $1 failed, please check if the version exists ${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/x-ui/ ]]; then
        systemctl stop x-ui
        rm /usr/local/x-ui/ -rf
    fi

    tar zxvf x-ui-linux-$(arch).tar.gz
    rm x-ui-linux-$(arch).tar.gz -f
    cd x-ui
    chmod +x x-ui

    # Check the system's architecture and rename the file accordingly
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm
        chmod +x bin/xray-linux-arm
    fi

    if [[ -f bin/xray-linux-$(arch) ]]; then
        chmod +x bin/xray-linux-$(arch)
    fi

    cp -f x-ui.service /etc/systemd/system/
    wget --no-check-certificate -O /usr/bin/x-ui https://raw.githubusercontent.com/Incognito-Coder/tx-ui/main/x-ui.sh
    chmod +x /usr/local/x-ui/x-ui.sh
    chmod +x /usr/bin/x-ui
    config_after_install

    systemctl daemon-reload
    systemctl enable x-ui
    systemctl start x-ui

    echo -e "${green}x-ui ${tag_version}${plain} installation finished, it is running now...\n"

    echo -e "┌───────────────────────────────────────────────────────┐"
    echo -e "│  ${blue}x-ui control menu usages (subcommands):${plain}              │"
    echo -e "│                                                       │"
    echo -e "│  ${blue}x-ui${plain}              - Admin Management Script          │"
    echo -e "│  ${blue}x-ui start${plain}        - Start                            │"
    echo -e "│  ${blue}x-ui stop${plain}         - Stop                             │"
    echo -e "│  ${blue}x-ui restart${plain}      - Restart                          │"
    echo -e "│  ${blue}x-ui status${plain}       - Current Status                   │"
    echo -e "│  ${blue}x-ui settings${plain}     - Current Settings                 │"
    echo -e "│  ${blue}x-ui enable${plain}       - Enable Autostart on OS Startup   │"
    echo -e "│  ${blue}x-ui disable${plain}      - Disable Autostart on OS Startup  │"
    echo -e "│  ${blue}x-ui log${plain}          - Check logs                       │"
    echo -e "│  ${blue}x-ui banlog${plain}       - Check Fail2ban ban logs          │"
    echo -e "│  ${blue}x-ui update${plain}       - Update                           │"
    echo -e "│  ${blue}x-ui legacy${plain}       - Legacy version                   │"
    echo -e "│  ${blue}x-ui install${plain}      - Install                          │"
    echo -e "│  ${blue}x-ui uninstall${plain}    - Uninstall                        │"
    echo -e "└───────────────────────────────────────────────────────┘"
}

echo -e "${green}Running...${plain}"
install_base
install_x-ui $1
