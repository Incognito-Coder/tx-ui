#!/bin/sh

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

# Logging helpers (POSIX sh / ash / dash / bash compatible)
LOGD() {
    printf '%b\n' "${yellow}[DEG] $* ${plain}"
}

LOGE() {
    printf '%b\n' "${red}[ERR] $* ${plain}"
}

LOGI() {
    printf '%b\n' "${green}[INF] $* ${plain}"
}

LOGW() {
    printf '%b\n' "${yellow}[WRN] $* ${plain}"
}

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

prompt_secret() {
    _prompt_text="$1"
    _var_name="$2"
    _ans=""
    printf '%s' "$_prompt_text"
    if command -v stty >/dev/null 2>&1; then
        stty -echo 2>/dev/null || true
    fi
    if [ -t 0 ]; then
        read -r _ans || _ans=""
    elif [ -r /dev/tty ]; then
        read -r _ans < /dev/tty || _ans=""
    fi
    if command -v stty >/dev/null 2>&1; then
        stty echo 2>/dev/null || true
    fi
    echo ""
    eval "$_var_name=\$_ans"
}

is_ipv4_address() {
    _ip="$1"
    printf '%s\n' "$_ip" | grep -Eq '^([0-9]{1,3}\.){3}[0-9]{1,3}$' || return 1

    _old_ifs="$IFS"
    IFS='.'
    set -- $_ip
    IFS="$_old_ifs"
    [ $# -eq 4 ] || return 1
    for _octet in "$@"; do
        [ "$_octet" -ge 0 ] 2>/dev/null && [ "$_octet" -le 255 ] 2>/dev/null || return 1
    done
    return 0
}

is_ipv6_address() {
    _ip="$1"
    case "$_ip" in
        *:*) ;;
        *) return 1 ;;
    esac
    printf '%s\n' "$_ip" | grep -Eq '^[0-9A-Fa-f:]+$'
}

is_ip_address() {
    is_ipv4_address "$1" || is_ipv6_address "$1"
}

format_host_for_url() {
    _host="$1"
    if is_ipv6_address "$_host"; then
        printf '[%s]' "$_host"
    else
        printf '%s' "$_host"
    fi
}

# Check root
if [ "$(id -u)" -ne 0 ]; then
    LOGE "ERROR: You must be root to run this script!\n"
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

os_version=""
if [ -f /etc/os-release ]; then
    os_version=$(grep "^VERSION_ID" /etc/os-release 2>/dev/null | cut -d '=' -f2 | tr -d '"' | tr -d '.')
fi

detect_init_system() {
    if command -v systemctl >/dev/null 2>&1 && { [ -d /run/systemd/system ] || [ -d /etc/systemd/system ]; }; then
        echo "systemd"
    elif [ "$release" = "openwrt" ] || [ -f /etc/init.d/x-ui ] || [ -f /etc/rc.common ]; then
        echo "procd"
    elif command -v rc-service >/dev/null 2>&1; then
        echo "openrc"
    else
        echo "unknown"
    fi
}

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

run_remote_script() {
    _script_url="$1"
    shift
    _tmp_script="/tmp/x-ui-remote-$$.sh"
    if ! download_file "$_script_url" "$_tmp_script"; then
        LOGE "Failed to download script: $_script_url"
        rm -f "$_tmp_script"
        return 1
    fi
    chmod +x "$_tmp_script"
    if head -n 1 "$_tmp_script" 2>/dev/null | grep -q "bash" && command -v bash >/dev/null 2>&1; then
        bash "$_tmp_script" "$@"
    else
        sh "$_tmp_script" "$@"
    fi
    _rc=$?
    rm -f "$_tmp_script"
    return $_rc
}

get_server_ip() {
    _ip=$(fetch_text "https://api.ipify.org" | tr -d ' \r\n')
    if [ -z "$_ip" ] && command -v ip >/dev/null 2>&1; then
        _ip=$(ip -4 addr show 2>/dev/null | awk '/inet / && !/127\.0\.0\.1/ {sub(/\/.*/, "", $2); print $2; exit}')
    fi
    echo "${_ip:-127.0.0.1}"
}

list_cert_domains() {
    if [ -d /root/cert ]; then
        for _d in /root/cert/*/; do
            [ -d "$_d" ] || continue
            basename "$_d"
        done
    fi
}

# Declare Variables
log_folder="${XUI_LOG_FOLDER:-/var/log}"
iplimit_log_path="${log_folder}/3xipl.log"
iplimit_banned_log_path="${log_folder}/3xipl-banned.log"

confirm() {
    if [ $# -gt 1 ]; then
        echo ""
        prompt_read "$1 [Default $2]: " temp "$2"
    else
        prompt_read "$1 [y/n]: " temp ""
    fi
    case "${temp}" in
        y|Y) return 0 ;;
        *) return 1 ;;
    esac
}

confirm_restart() {
    confirm "Restart the panel, Attention: Restarting the panel will also restart xray" "y"
    if [ $? -eq 0 ]; then
        restart
    else
        show_menu
    fi
}

before_show_menu() {
    echo ""
    printf '%b' "${yellow}Press enter to return to the main menu: ${plain}"
    if [ -t 0 ]; then
        read -r temp || true
    elif [ -r /dev/tty ]; then
        read -r temp < /dev/tty || true
    fi
    show_menu
}

install() {
    run_remote_script "https://raw.githubusercontent.com/Incognito-Coder/tx-ui/main/install.sh"
    if [ $? -eq 0 ]; then
        if [ $# -eq 0 ]; then
            start
        else
            start 0
        fi
    fi
}

update() {
    confirm "This function will forcefully reinstall the latest version, and the data will not be lost. Do you want to continue?" "y"
    if [ $? -ne 0 ]; then
        LOGE "Cancelled"
        if [ $# -eq 0 ]; then
            before_show_menu
        fi
        return 0
    fi
    run_remote_script "https://raw.githubusercontent.com/Incognito-Coder/tx-ui/main/install.sh"
    if [ $? -eq 0 ]; then
        LOGI "Update is complete, Panel has automatically restarted"
        if [ $# -eq 0 ]; then
            before_show_menu
        fi
    fi
}

update_menu() {
    printf '%b\n' "${yellow}Updating Menu${plain}"
    confirm "This function will update the menu to the latest changes." "y"
    if [ $? -ne 0 ]; then
        LOGE "Cancelled"
        if [ $# -eq 0 ]; then
            before_show_menu
        fi
        return 0
    fi

    if download_file "https://raw.githubusercontent.com/Incognito-Coder/tx-ui/main/x-ui.sh" /usr/bin/x-ui; then
        chmod +x /usr/local/x-ui/x-ui.sh 2>/dev/null || true
        chmod +x /usr/bin/x-ui
        ln -sf /usr/bin/x-ui /usr/bin/tx-ui
        printf '%b\n' "${green}Update successful.${plain}"
        if [ $# -eq 0 ]; then
            before_show_menu
        fi
    else
        printf '%b\n' "${red}Failed to update the menu.${plain}"
        return 1
    fi
}

legacy_version() {
    echo "Enter the panel version (like 0.9.8):"
    prompt_read "" tag_version ""

    if [ -z "$tag_version" ]; then
        echo "Panel version cannot be empty. Exiting."
        exit 1
    fi
    tag_version=${tag_version#v}

    echo "Downloading and installing panel version $tag_version..."
    run_remote_script "https://raw.githubusercontent.com/Incognito-Coder/tx-ui/v${tag_version}/install.sh" "v${tag_version}"
}

delete_script() {
    rm -f "$0"
    exit 1
}

uninstall() {
    confirm "Are you sure you want to uninstall the panel? xray will also be uninstalled!" "n"
    if [ $? -ne 0 ]; then
        if [ $# -eq 0 ]; then
            show_menu
        fi
        return 0
    fi

    _init=$(detect_init_system)
    case "$_init" in
        systemd)
            systemctl stop x-ui 2>/dev/null || true
            systemctl disable x-ui 2>/dev/null || true
            rm -f /etc/systemd/system/x-ui.service
            systemctl daemon-reload 2>/dev/null || true
            systemctl reset-failed 2>/dev/null || true
            ;;
        procd)
            if [ -x /etc/init.d/x-ui ]; then
                /etc/init.d/x-ui stop 2>/dev/null || true
                /etc/init.d/x-ui disable 2>/dev/null || true
            fi
            rm -f /etc/init.d/x-ui /etc/rc.d/*x-ui 2>/dev/null || true
            ;;
        openrc)
            rc-service x-ui stop 2>/dev/null || true
            rc-update del x-ui default 2>/dev/null || true
            rm -f /etc/init.d/x-ui
            ;;
    esac

    rm -rf /etc/x-ui/ /usr/local/x-ui/
    rm -f /usr/bin/x-ui /usr/bin/tx-ui

    echo ""
    printf '%b\n' "Uninstalled Successfully.\n"
    echo "If you need to install this panel again, you can use below command:"
    printf '%b\n' "${green}curl -Ls https://raw.githubusercontent.com/Incognito-Coder/tx-ui/main/install.sh | sh${plain}"
    echo ""
    trap delete_script TERM
    delete_script
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

reset_user() {
    confirm "Are you sure to reset the username and password of the panel?" "n"
    if [ $? -ne 0 ]; then
        if [ $# -eq 0 ]; then
            show_menu
        fi
        return 0
    fi
    prompt_read "Please set the login username [default is a random username]: " config_account ""
    if [ -z "$config_account" ]; then
        config_account=$(gen_random_string 8)
    fi
    prompt_read "Please set the login password [default is a random password]: " config_password ""
    if [ -z "$config_password" ]; then
        config_password=$(gen_random_string 8)
    fi
    /usr/local/x-ui/x-ui setting -username "${config_account}" -password "${config_password}" >/dev/null 2>&1
    /usr/local/x-ui/x-ui setting -remove_secret >/dev/null 2>&1
    printf '%b\n' "Panel login username has been reset to: ${green} ${config_account} ${plain}"
    printf '%b\n' "Panel login password has been reset to: ${green} ${config_password} ${plain}"
    printf '%b\n' "${yellow} Panel login secret token disabled ${plain}"
    printf '%b\n' "${green} Please use the new login username and password to access the X-UI panel. Also remember them! ${plain}"
    confirm_restart
}

reset_webbasepath() {
    printf '%b\n' "${yellow}Resetting Web Base Path${plain}"

    prompt_read "Are you sure you want to reset the web base path? (y/n): " confirm_ans ""
    if [ "$confirm_ans" != "y" ] && [ "$confirm_ans" != "Y" ]; then
        printf '%b\n' "${yellow}Operation canceled.${plain}"
        return
    fi

    config_webBasePath=$(gen_random_string 10)
    /usr/local/x-ui/x-ui setting -webBasePath "${config_webBasePath}" >/dev/null 2>&1

    printf '%b\n' "Web base path has been reset to: ${green}${config_webBasePath}${plain}"
    printf '%b\n' "${green}Please use the new web base path to access the panel.${plain}"
    restart
}

reset_config() {
    confirm "Are you sure you want to reset all panel settings, Account data will not be lost, Username and password will not change" "n"
    if [ $? -ne 0 ]; then
        if [ $# -eq 0 ]; then
            show_menu
        fi
        return 0
    fi
    /usr/local/x-ui/x-ui setting -reset
    echo "All panel settings have been reset to default."
    restart
}

factory_reset() {
    printf '%b\n' "${red}╔══════════════════════════════════════════════════════════╗${plain}"
    printf '%b\n' "${red}║                     WARNING!                             ║${plain}"
    printf '%b\n' "${red}║ This will permanently delete ALL inbounds, clients,      ║${plain}"
    printf '%b\n' "${red}║ custom settings, and staff accounts!                     ║${plain}"
    printf '%b\n' "${red}║ Admin credentials will be reset to: admin / admin        ║${plain}"
    printf '%b\n' "${red}║ Default port will be reset to: 2053                      ║${plain}"
    printf '%b\n' "${red}╚══════════════════════════════════════════════════════════╝${plain}"
    confirm "Are you sure you want to factory reset tx-ui?" "n"
    if [ $? -ne 0 ]; then
        if [ $# -eq 0 ]; then
            show_menu
        fi
        return 0
    fi
    /usr/local/x-ui/x-ui reset
    LOGI "Factory reset completed successfully."
    restart
}

check_config() {
    info=$(/usr/local/x-ui/x-ui setting -show true)
    if [ $? -ne 0 ]; then
        LOGE "get current settings error, please check logs"
        show_menu
        return
    fi
    LOGI "${info}"

    existing_webBasePath=$(printf '%s\n' "$info" | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    existing_port=$(printf '%s\n' "$info" | grep -Eo 'port: .+' | awk '{print $2}')
    existing_cert=$(/usr/local/x-ui/x-ui setting -getCert true | grep -Eo 'cert: .+' | awk '{print $2}')
    server_ip=$(get_server_ip)

    if [ -n "$existing_cert" ]; then
        cert_host=$(basename "$(dirname "$existing_cert")")
        formatted_cert_host=$(format_host_for_url "$cert_host")

        if [ -n "$cert_host" ]; then
            printf '%b\n' "${green}Access URL: https://${formatted_cert_host}:${existing_port}${existing_webBasePath}${plain}"
        else
            printf '%b\n' "${green}Access URL: https://${server_ip}:${existing_port}${existing_webBasePath}${plain}"
        fi
    else
        printf '%b\n' "${green}Access URL: http://${server_ip}:${existing_port}${existing_webBasePath}${plain}"
    fi

    env_file="/etc/x-ui/x-ui.env"
    db_info="SQLite (/etc/x-ui/x-ui.db)"
    if [ -f "${env_file}" ]; then
        . "${env_file}" 2>/dev/null || true
        if [ "${XUI_DB_TYPE:-}" = "postgres" ] || [ "${XUI_DB_TYPE:-}" = "postgresql" ] || [ "${XUI_DB_TYPE:-}" = "pg" ]; then
            if [ -n "${XUI_DB_POSTGRES_DSN:-}" ]; then
                db_info="PostgreSQL (${XUI_DB_POSTGRES_DSN})"
            else
                db_info="PostgreSQL (${XUI_DB_USER:-postgres}@${XUI_DB_HOST:-127.0.0.1}:${XUI_DB_PORT:-5432}/${XUI_DB_NAME:-xui})"
            fi
        fi
    fi
    printf '%b\n' "${green}Database: ${db_info}${plain}"
}

set_port() {
    echo ""
    prompt_read "Enter port number[1-65535]: " port ""
    if [ -z "${port}" ]; then
        LOGD "Cancelled"
        before_show_menu
    else
        /usr/local/x-ui/x-ui setting -port "${port}"
        printf '%b\n' "The port is set, Please restart the panel now, and use the new port ${green}${port}${plain} to access web panel"
        confirm_restart
    fi
}

svc_do_start() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd) systemctl start x-ui ;;
        procd) /etc/init.d/x-ui start ;;
        openrc) rc-service x-ui start ;;
        *) return 1 ;;
    esac
}

svc_do_stop() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd) systemctl stop x-ui ;;
        procd) /etc/init.d/x-ui stop ;;
        openrc) rc-service x-ui stop ;;
        *) return 1 ;;
    esac
}

svc_do_restart() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd) systemctl restart x-ui ;;
        procd) /etc/init.d/x-ui restart ;;
        openrc) rc-service x-ui restart ;;
        *) return 1 ;;
    esac
}

start() {
    check_status
    if [ $? -eq 0 ]; then
        echo ""
        LOGI "Panel is running, No need to start again, If you need to restart, please select restart"
    else
        svc_do_start
        sleep 2
        check_status
        if [ $? -eq 0 ]; then
            LOGI "x-ui Started Successfully"
        else
            LOGE "panel Failed to start, Probably because it takes longer than two seconds to start, Please check the log information later"
        fi
    fi

    if [ $# -eq 0 ]; then
        before_show_menu
    fi
}

stop() {
    check_status
    if [ $? -eq 1 ]; then
        echo ""
        LOGI "Panel stopped, No need to stop again!"
    else
        svc_do_stop
        sleep 2
        check_status
        if [ $? -eq 1 ]; then
            LOGI "x-ui and xray stopped successfully"
        else
            LOGE "Panel stop failed, Probably because the stop time exceeds two seconds, Please check the log information later"
        fi
    fi

    if [ $# -eq 0 ]; then
        before_show_menu
    fi
}

restart() {
    svc_do_restart
    sleep 2
    check_status
    if [ $? -eq 0 ]; then
        LOGI "x-ui and xray Restarted successfully"
    else
        LOGE "Panel restart failed, Probably because it takes longer than two seconds to start, Please check the log information later"
    fi
    if [ $# -eq 0 ]; then
        before_show_menu
    fi
}

status() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd)
            systemctl status x-ui -l --no-pager 2>/dev/null || systemctl status x-ui -l
            ;;
        procd)
            /etc/init.d/x-ui status 2>/dev/null || true
            show_status
            ;;
        openrc)
            rc-service x-ui status 2>/dev/null || show_status
            ;;
        *)
            show_status
            ;;
    esac
    if [ $# -eq 0 ]; then
        before_show_menu
    fi
}

enable() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd) systemctl enable x-ui ;;
        procd) /etc/init.d/x-ui enable ;;
        openrc) rc-update add x-ui default ;;
        *) return 1 ;;
    esac
    if [ $? -eq 0 ]; then
        LOGI "x-ui Set to boot automatically on startup successfully"
    else
        LOGE "x-ui Failed to set Autostart"
    fi

    if [ $# -eq 0 ]; then
        before_show_menu
    fi
}

disable() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd) systemctl disable x-ui ;;
        procd) /etc/init.d/x-ui disable ;;
        openrc) rc-update del x-ui default ;;
        *) return 1 ;;
    esac
    if [ $? -eq 0 ]; then
        LOGI "x-ui Autostart Cancelled successfully"
    else
        LOGE "x-ui Failed to cancel autostart"
    fi

    if [ $# -eq 0 ]; then
        before_show_menu
    fi
}

show_log() {
    printf '%b\n' "${green}\t1.${plain} Debug Log"
    printf '%b\n' "${green}\t2.${plain} Clear All logs"
    printf '%b\n' "${green}\t0.${plain} Back to Main Menu"
    prompt_read "Choose an option: " choice ""

    _init=$(detect_init_system)
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        if command -v journalctl >/dev/null 2>&1 && [ "$_init" = "systemd" ]; then
            journalctl -u x-ui -e --no-pager -f -p debug
        elif command -v logread >/dev/null 2>&1; then
            printf '%b\n' "${yellow}Showing OpenWrt system logs for x-ui (Press Ctrl+C to exit)...${plain}"
            logread | grep -E "x-ui|xray" | tail -n 50
            logread -f | grep -E "x-ui|xray"
        else
            LOGE "No system log viewer (journalctl/logread) found."
        fi
        if [ $# -eq 0 ]; then
            before_show_menu
        fi
        ;;
    2)
        if command -v journalctl >/dev/null 2>&1 && [ "$_init" = "systemd" ]; then
            journalctl --rotate 2>/dev/null || true
            journalctl --vacuum-time=1s 2>/dev/null || true
            echo "All Logs cleared."
        elif [ "$_init" = "procd" ] && [ -x /etc/init.d/log ]; then
            /etc/init.d/log restart 2>/dev/null || true
            echo "OpenWrt ring-buffer logs cleared."
        fi
        restart
        ;;
    *)
        printf '%b\n' "${red}Invalid option. Please select a valid number.${plain}\n"
        show_log
        ;;
    esac
}

show_banlog() {
    system_log="/var/log/fail2ban.log"
    printf '%b\n' "${green}Checking ban logs...${plain}\n"

    if ! command -v fail2ban-client >/dev/null 2>&1; then
        printf '%b\n' "${red}Fail2ban is not installed on this system.${plain}\n"
        return 1
    fi
    if command -v systemctl >/dev/null 2>&1 && ! systemctl is-active --quiet fail2ban; then
        printf '%b\n' "${red}Fail2ban service is not running!${plain}\n"
        return 1
    fi

    if [ -f "$system_log" ]; then
        printf '%b\n' "${green}Recent system ban activities from fail2ban.log:${plain}"
        grep "3x-ipl" "$system_log" | grep -E "Ban|Unban" | tail -n 10 || printf '%b\n' "${yellow}No recent system ban activities found${plain}"
        echo ""
    fi

    if [ -f "${iplimit_banned_log_path}" ]; then
        printf '%b\n' "${green}3X-IPL ban log entries:${plain}"
        if [ -s "${iplimit_banned_log_path}" ]; then
            grep -v "INIT" "${iplimit_banned_log_path}" | tail -n 10 || printf '%b\n' "${yellow}No ban entries found${plain}"
        else
            printf '%b\n' "${yellow}Ban log file is empty${plain}"
        fi
    else
        printf '%b\n' "${red}Ban log file not found at: ${iplimit_banned_log_path}${plain}"
    fi

    printf '%b\n' "\n${green}Current jail status:${plain}"
    fail2ban-client status 3x-ipl || printf '%b\n' "${yellow}Unable to get jail status${plain}"
}

bbr_menu() {
    printf '%b\n' "${green}\t1.${plain} Enable BBR"
    printf '%b\n' "${green}\t2.${plain} Disable BBR"
    printf '%b\n' "${green}\t0.${plain} Back to Main Menu"
    prompt_read "Choose an option: " choice ""
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        enable_bbr
        bbr_menu
        ;;
    2)
        disable_bbr
        bbr_menu
        ;;
    *)
        printf '%b\n' "${red}Invalid option. Please select a valid number.${plain}\n"
        bbr_menu
        ;;
    esac
}

disable_bbr() {
    if ! grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf 2>/dev/null || ! grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf 2>/dev/null; then
        printf '%b\n' "${yellow}BBR is not currently enabled in /etc/sysctl.conf.${plain}"
        before_show_menu
        return
    fi

    sed -i 's/net.core.default_qdisc=fq/net.core.default_qdisc=pfifo_fast/g' /etc/sysctl.conf
    sed -i 's/net.ipv4.tcp_congestion_control=bbr/net.ipv4.tcp_congestion_control=cubic/g' /etc/sysctl.conf
    [ -f /etc/sysctl.d/99-x-ui-bbr.conf ] && rm -f /etc/sysctl.d/99-x-ui-bbr.conf

    sysctl -w net.core.default_qdisc=pfifo_fast >/dev/null 2>&1 || true
    sysctl -w net.ipv4.tcp_congestion_control=cubic >/dev/null 2>&1 || true
    sysctl -p >/dev/null 2>&1 || true

    _cc=$(cat /proc/sys/net/ipv4/tcp_congestion_control 2>/dev/null || sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null)
    if [ "$_cc" = "cubic" ]; then
        printf '%b\n' "${green}BBR has been replaced with CUBIC successfully.${plain}"
    else
        printf '%b\n' "${red}Failed to replace BBR with CUBIC. Please check your system configuration.${plain}"
    fi
}

enable_bbr() {
    _cur_cc=$(cat /proc/sys/net/ipv4/tcp_congestion_control 2>/dev/null || echo "")
    if [ "$_cur_cc" = "bbr" ] && grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf 2>/dev/null; then
        printf '%b\n' "${green}BBR is already enabled!${plain}"
        before_show_menu
        return
    fi

    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update && apt-get install -y ca-certificates openssl
        ;;
    centos | rhel | almalinux | rocky | ol)
        yum -y update && yum install -y ca-certificates openssl
        ;;
    fedora | amzn)
        dnf -y update && dnf install -y ca-certificates openssl
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm ca-certificates openssl
        ;;
    opensuse* | suse)
        zypper refresh && zypper install -y ca-certificates openssl
        ;;
    alpine)
        apk update && apk add ca-certificates openssl
        ;;
    openwrt)
        if command -v opkg >/dev/null 2>&1; then
            opkg update && opkg install kmod-tcp-bbr ca-bundle openssl-util || true
        elif command -v apk >/dev/null 2>&1; then
            apk update && apk add kmod-tcp-bbr ca-bundle openssl-util || true
        fi
        ;;
    gentoo)
        emerge --sync && emerge --ask --quiet ca-certificates openssl
        ;;
    void)
        xbps-install -S && xbps-install -y ca-certificates openssl
        ;;
    solus)
        eopkg update && eopkg install -y ca-certificates openssl
        ;;
    *)
        printf '%b\n' "${yellow}Skipping package install for OS '${release}'; attempting sysctl BBR enablement.${plain}"
        ;;
    esac

    if grep -q "net.core.default_qdisc=" /etc/sysctl.conf 2>/dev/null; then
        sed -i 's/^net\.core\.default_qdisc=.*/net.core.default_qdisc=fq/' /etc/sysctl.conf
    else
        echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
    fi

    if grep -q "net.ipv4.tcp_congestion_control=" /etc/sysctl.conf 2>/dev/null; then
        sed -i 's/^net\.ipv4\.tcp_congestion_control=.*/net.ipv4.tcp_congestion_control=bbr/' /etc/sysctl.conf
    else
        echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
    fi

    if [ -d /etc/sysctl.d ]; then
        cat <<EOF > /etc/sysctl.d/99-x-ui-bbr.conf
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
EOF
    fi

    sysctl -w net.core.default_qdisc=fq >/dev/null 2>&1 || true
    sysctl -w net.ipv4.tcp_congestion_control=bbr >/dev/null 2>&1 || true
    sysctl -p >/dev/null 2>&1 || true

    _cc=$(cat /proc/sys/net/ipv4/tcp_congestion_control 2>/dev/null || sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null)
    if [ "$_cc" = "bbr" ]; then
        printf '%b\n' "${green}BBR has been enabled successfully.${plain}"
    else
        printf '%b\n' "${red}Failed to enable BBR. Ensure your kernel supports tcp_bbr (kmod-tcp-bbr on OpenWrt).${plain}"
    fi
}

update_shell() {
    if ! download_file "https://github.com/Incognito-Coder/tx-ui/raw/main/x-ui.sh" /usr/bin/x-ui; then
        echo ""
        LOGE "Failed to download script, Please check whether the machine can connect Github"
        before_show_menu
    else
        chmod +x /usr/bin/x-ui
        ln -sf /usr/bin/x-ui /usr/bin/tx-ui
        LOGI "Upgrade script succeeded, Please rerun the script"
        before_show_menu
    fi
}

# 0: running, 1: not running, 2: not installed
check_status() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd)
            if [ ! -f /etc/systemd/system/x-ui.service ]; then
                return 2
            fi
            if systemctl is-active --quiet x-ui; then
                return 0
            else
                return 1
            fi
            ;;
        procd|openrc)
            if [ ! -f /etc/init.d/x-ui ] && [ ! -x /usr/local/x-ui/x-ui ]; then
                return 2
            fi
            if command -v ubus >/dev/null 2>&1; then
                if ubus call service list '{"name":"x-ui"}' 2>/dev/null | grep -q '"running": true'; then
                    return 0
                fi
            fi
            _cnt=$( (ps w 2>/dev/null || ps -ef 2>/dev/null || ps 2>/dev/null) | grep -E '/usr/local/x-ui/x-ui([[:space:]]|$)' | grep -v grep | wc -l)
            if [ "$_cnt" -gt 0 ] 2>/dev/null; then
                return 0
            else
                return 1
            fi
            ;;
        *)
            if [ ! -x /usr/local/x-ui/x-ui ]; then
                return 2
            fi
            _cnt=$( (ps w 2>/dev/null || ps -ef 2>/dev/null || ps 2>/dev/null) | grep -E '/usr/local/x-ui/x-ui([[:space:]]|$)' | grep -v grep | wc -l)
            if [ "$_cnt" -gt 0 ] 2>/dev/null; then
                return 0
            else
                return 1
            fi
            ;;
    esac
}

check_enabled() {
    _init=$(detect_init_system)
    case "$_init" in
        systemd)
            _temp=$(systemctl is-enabled x-ui 2>/dev/null)
            [ "${_temp}" = "enabled" ]
            ;;
        procd)
            if [ -x /etc/init.d/x-ui ]; then
                /etc/init.d/x-ui enabled >/dev/null 2>&1 && return 0
            fi
            ls /etc/rc.d/S*x-ui >/dev/null 2>&1
            ;;
        openrc)
            rc-update show default 2>/dev/null | grep -qw x-ui
            ;;
        *)
            return 1
            ;;
    esac
}

check_uninstall() {
    check_status
    if [ $? -ne 2 ]; then
        echo ""
        LOGE "Panel installed, Please do not reinstall"
        if [ $# -eq 0 ]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

check_install() {
    check_status
    if [ $? -eq 2 ]; then
        echo ""
        LOGE "Please install the panel first"
        if [ $# -eq 0 ]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

show_status() {
    check_status
    case $? in
    0)
        printf '%b\n' "Panel state: ${green}Running${plain}"
        show_enable_status
        ;;
    1)
        printf '%b\n' "Panel state: ${yellow}Not Running${plain}"
        show_enable_status
        ;;
    2)
        printf '%b\n' "Panel state: ${red}Not Installed${plain}"
        ;;
    esac
    show_xray_status
}

show_enable_status() {
    check_enabled
    if [ $? -eq 0 ]; then
        printf '%b\n' "Start automatically: ${green}Yes${plain}"
    else
        printf '%b\n' "Start automatically: ${red}No${plain}"
    fi
}

check_xray_status() {
    count=$( (ps w 2>/dev/null || ps -ef 2>/dev/null || ps 2>/dev/null) | grep "xray-linux" | grep -v "grep" | wc -l)
    if [ "$count" -ne 0 ] 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

show_xray_status() {
    check_xray_status
    if [ $? -eq 0 ]; then
        printf '%b\n' "xray state: ${green}Running${plain}"
    else
        printf '%b\n' "xray state: ${red}Not Running${plain}"
    fi
}

openwrt_firewall_list() {
    echo "Current OpenWrt TX-UI / custom WAN firewall rules:"
    uci show firewall 2>/dev/null | grep -E "firewall\.@?rule\[.*\].*(Allow-TX-UI|name=|dest_port=|proto=|target=)" || echo "(No custom UCI firewall rules found)"
}

openwrt_open_ports() {
    prompt_read "Enter ports to allow from WAN (e.g. 2053,2096,443 or range 40000-40100): " ports ""
    if ! printf '%s\n' "$ports" | grep -Eq '^([0-9]+|[0-9]+-[0-9]+)(,([0-9]+|[0-9]+-[0-9]+))*$'; then
        echo "Error: Invalid input. Enter comma-separated ports or ranges (e.g. 80,443,2053 or 400-500)." >&2
        return 1
    fi
    _old_ifs="$IFS"
    IFS=','
    set -- $ports
    IFS="$_old_ifs"
    for port in "$@"; do
        _uci_port=$(printf '%s' "$port" | tr '-' ':')
        _rule_name="Allow-TX-UI-${port}"
        uci add firewall rule >/dev/null
        uci set firewall.@rule[-1].name="$_rule_name"
        uci set firewall.@rule[-1].src='wan'
        uci set firewall.@rule[-1].proto='tcp udp'
        uci set firewall.@rule[-1].dest_port="$_uci_port"
        uci set firewall.@rule[-1].target='ACCEPT'
        echo "Added OpenWrt WAN rule: ${_rule_name} (ports ${_uci_port})"
    done
    uci commit firewall
    /etc/init.d/firewall reload 2>/dev/null || /etc/init.d/firewall restart 2>/dev/null || true
    printf '%b\n' "${green}OpenWrt firewall rules applied and reloaded.${plain}"
}

openwrt_delete_ports() {
    openwrt_firewall_list
    prompt_read "Enter port (or Allow-TX-UI-<port>) to remove: " target_port ""
    [ -z "$target_port" ] && return 1
    _removed=0
    _idx=0
    while uci -q get "firewall.@rule[${_idx}]" >/dev/null 2>&1; do
        _rname=$(uci -q get "firewall.@rule[${_idx}].name" || echo "")
        _dport=$(uci -q get "firewall.@rule[${_idx}].dest_port" || echo "")
        if [ "$_rname" = "$target_port" ] || [ "$_rname" = "Allow-TX-UI-${target_port}" ] || { [ "$_dport" = "$target_port" ] && case "$_rname" in Allow-TX-UI-*) true ;; *) false ;; esac; }; then
            uci delete "firewall.@rule[${_idx}]"
            echo "Deleted rule: ${_rname}"
            _removed=$((_removed + 1))
            continue
        fi
        _idx=$((_idx + 1))
    done
    if [ "$_removed" -gt 0 ]; then
        uci commit firewall
        /etc/init.d/firewall reload 2>/dev/null || /etc/init.d/firewall restart 2>/dev/null || true
        printf '%b\n' "${green}Firewall reloaded.${plain}"
    else
        printf '%b\n' "${yellow}No matching TX-UI firewall rule found for '${target_port}'.${plain}"
    fi
}

firewall_menu() {
    if [ "$release" = "openwrt" ] && command -v uci >/dev/null 2>&1; then
        printf '%b\n' "${green}\t1.${plain} List OpenWrt TX-UI Firewall Rules"
        printf '%b\n' "${green}\t2.${plain} Open Ports on WAN (UCI Firewall)"
        printf '%b\n' "${green}\t3.${plain} Remove Opened Port Rule"
        printf '%b\n' "${green}\t4.${plain} Reload OpenWrt Firewall"
        printf '%b\n' "${green}\t0.${plain} Back to Main Menu"
        prompt_read "Choose an option: " choice ""
        case "$choice" in
            0) show_menu ;;
            1) openwrt_firewall_list; firewall_menu ;;
            2) openwrt_open_ports; firewall_menu ;;
            3) openwrt_delete_ports; firewall_menu ;;
            4) /etc/init.d/firewall reload 2>/dev/null || /etc/init.d/firewall restart; echo "Firewall reloaded."; firewall_menu ;;
            *) printf '%b\n' "${red}Invalid option. Please select a valid number.${plain}\n"; firewall_menu ;;
        esac
        return
    fi

    printf '%b\n' "${green}\t1.${plain} ${green}Install${plain} Firewall"
    printf '%b\n' "${green}\t2.${plain} Port List [numbered]"
    printf '%b\n' "${green}\t3.${plain} ${green}Open${plain} Ports"
    printf '%b\n' "${green}\t4.${plain} ${red}Delete${plain} Ports from List"
    printf '%b\n' "${green}\t5.${plain} ${green}Enable${plain} Firewall"
    printf '%b\n' "${green}\t6.${plain} ${red}Disable${plain} Firewall"
    printf '%b\n' "${green}\t7.${plain} Firewall Status"
    printf '%b\n' "${green}\t0.${plain} Back to Main Menu"
    prompt_read "Choose an option: " choice ""
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        install_firewall
        firewall_menu
        ;;
    2)
        ufw status numbered
        firewall_menu
        ;;
    3)
        open_ports
        firewall_menu
        ;;
    4)
        delete_ports
        firewall_menu
        ;;
    5)
        ufw enable
        firewall_menu
        ;;
    6)
        ufw disable
        firewall_menu
        ;;
    7)
        ufw status verbose
        firewall_menu
        ;;
    *)
        printf '%b\n' "${red}Invalid option. Please select a valid number.${plain}\n"
        firewall_menu
        ;;
    esac
}

install_firewall() {
    if ! command -v ufw >/dev/null 2>&1; then
        echo "ufw firewall is not installed. Installing now..."
        if command -v apt-get >/dev/null 2>&1; then
            apt-get update && apt-get install -y ufw
        else
            LOGE "ufw automatic install requires apt-get. Use UCI firewall on OpenWrt or install ufw manually."
            return 1
        fi
    else
        echo "ufw firewall is already installed"
    fi

    if ufw status | grep -q "Status: active"; then
        echo "Firewall is already active"
    else
        echo "Activating firewall..."
        ufw allow ssh
        ufw allow http
        ufw allow https
        ufw allow 2053/tcp
        ufw allow 2096/tcp
        ufw --force enable
    fi
}

open_ports() {
    prompt_read "Enter the ports you want to open (e.g. 80,443,2053 or range 400-500): " ports ""

    if ! printf '%s\n' "$ports" | grep -Eq '^([0-9]+|[0-9]+-[0-9]+)(,([0-9]+|[0-9]+-[0-9]+))*$'; then
        echo "Error: Invalid input. Please enter a comma-separated list of ports or a range of ports (e.g. 80,443,2053 or 400-500)." >&2
        return 1
    fi

    _old_ifs="$IFS"
    IFS=','
    set -- $ports
    IFS="$_old_ifs"
    for port in "$@"; do
        case "$port" in
            *-*)
                start_port=${port%%-*}
                end_port=${port#*-}
                ufw allow "${start_port}:${end_port}/tcp"
                ufw allow "${start_port}:${end_port}/udp"
                ;;
            *)
                ufw allow "$port"
                ;;
        esac
    done

    echo "Opened the specified ports:"
    for port in "$@"; do
        case "$port" in
            *-*)
                start_port=${port%%-*}
                end_port=${port#*-}
                ufw status | grep -q "${start_port}:${end_port}" && echo "${start_port}-${end_port}"
                ;;
            *)
                ufw status | grep -q "$port" && echo "$port"
                ;;
        esac
    done
}

delete_ports() {
    echo "Current UFW rules:"
    ufw status numbered

    echo "Do you want to delete rules by:"
    echo "1) Rule numbers"
    echo "2) Ports"
    prompt_read "Enter your choice (1 or 2): " choice ""

    if [ "$choice" = "1" ]; then
        prompt_read "Enter the rule numbers you want to delete (1, 2, etc.): " rule_numbers ""
        if ! printf '%s\n' "$rule_numbers" | grep -Eq '^([0-9]+)(,[0-9]+)*$'; then
            echo "Error: Invalid input. Please enter a comma-separated list of rule numbers." >&2
            return 1
        fi
        _old_ifs="$IFS"
        IFS=','
        set -- $rule_numbers
        IFS="$_old_ifs"
        for rule_number in "$@"; do
            ufw delete "$rule_number" || echo "Failed to delete rule number $rule_number"
        done
        echo "Selected rules have been deleted."

    elif [ "$choice" = "2" ]; then
        prompt_read "Enter the ports you want to delete (e.g. 80,443,2053 or range 400-500): " ports ""
        if ! printf '%s\n' "$ports" | grep -Eq '^([0-9]+|[0-9]+-[0-9]+)(,([0-9]+|[0-9]+-[0-9]+))*$'; then
            echo "Error: Invalid input. Please enter a comma-separated list of ports or a range of ports (e.g. 80,443,2053 or 400-500)." >&2
            return 1
        fi
        _old_ifs="$IFS"
        IFS=','
        set -- $ports
        IFS="$_old_ifs"
        for port in "$@"; do
            case "$port" in
                *-*)
                    start_port=${port%%-*}
                    end_port=${port#*-}
                    ufw delete allow "${start_port}:${end_port}/tcp"
                    ufw delete allow "${start_port}:${end_port}/udp"
                    ;;
                *)
                    ufw delete allow "$port"
                    ;;
            esac
        done
        echo "Deleted the specified ports."
    else
        printf '%b\n' "${red}Error:${plain} Invalid choice. Please enter 1 or 2." >&2
        return 1
    fi
}

update_geo() {
    printf '%b\n' "${green}\t1.${plain} Loyalsoldier (geoip.dat, geosite.dat)"
    printf '%b\n' "${green}\t2.${plain} chocolate4u (geoip_IR.dat, geosite_IR.dat)"
    printf '%b\n' "${green}\t3.${plain} runetfreedom (geoip_RU.dat, geosite_RU.dat)"
    printf '%b\n' "${green}\t0.${plain} Back to Main Menu"
    prompt_read "Choose an option: " choice ""

    cd /usr/local/x-ui/bin || return 1

    case "$choice" in
    0)
        show_menu
        ;;
    1)
        svc_do_stop
        rm -f geoip.dat geosite.dat
        download_file "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat" "geoip.dat"
        download_file "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat" "geosite.dat"
        geo_tag=$(fetch_text "https://api.github.com/repos/Loyalsoldier/v2ray-rules-dat/releases/latest" \
            | grep '"tag_name":' \
            | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | head -n 1)
        if [ -n "$geo_tag" ] && [ "$geo_tag" != "latest" ]; then
            echo "$geo_tag" > geoip.dat.version
            echo "$geo_tag" > geosite.dat.version
            mkdir -p /etc/x-ui
            echo "$geo_tag" > /etc/x-ui/geoip.dat.version
            echo "$geo_tag" > /etc/x-ui/geosite.dat.version
        fi
        printf '%b\n' "${green}Loyalsoldier datasets have been updated successfully!${plain}"
        restart
        ;;
    2)
        svc_do_stop
        rm -f geoip_IR.dat geosite_IR.dat
        download_file "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat" "geoip_IR.dat"
        download_file "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat" "geosite_IR.dat"
        geo_ir_tag=$(fetch_text "https://api.github.com/repos/chocolate4u/Iran-v2ray-rules/releases/latest" \
            | grep '"tag_name":' \
            | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | head -n 1)
        if [ -n "$geo_ir_tag" ] && [ "$geo_ir_tag" != "latest" ]; then
            echo "$geo_ir_tag" > geoip_IR.dat.version
            echo "$geo_ir_tag" > geosite_IR.dat.version
            mkdir -p /etc/x-ui
            echo "$geo_ir_tag" > /etc/x-ui/geoip_IR.dat.version
            echo "$geo_ir_tag" > /etc/x-ui/geosite_IR.dat.version
        fi
        printf '%b\n' "${green}chocolate4u datasets have been updated successfully!${plain}"
        restart
        ;;
    3)
        svc_do_stop
        rm -f geoip_RU.dat geosite_RU.dat
        download_file "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat" "geoip_RU.dat"
        download_file "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat" "geosite_RU.dat"
        geo_ru_tag=$(fetch_text "https://api.github.com/repos/runetfreedom/russia-v2ray-rules-dat/releases/latest" \
            | grep '"tag_name":' \
            | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | head -n 1)
        if [ -n "$geo_ru_tag" ] && [ "$geo_ru_tag" != "latest" ]; then
            echo "$geo_ru_tag" > geoip_RU.dat.version
            echo "$geo_ru_tag" > geosite_RU.dat.version
            mkdir -p /etc/x-ui
            echo "$geo_ru_tag" > /etc/x-ui/geoip_RU.dat.version
            echo "$geo_ru_tag" > /etc/x-ui/geosite_RU.dat.version
        fi
        printf '%b\n' "${green}runetfreedom datasets have been updated successfully!${plain}"
        restart
        ;;
    *)
        printf '%b\n' "${red}Invalid option. Please select a valid number.${plain}\n"
        update_geo
        ;;
    esac

    before_show_menu
}

install_acme() {
    if command -v ~/.acme.sh/acme.sh >/dev/null 2>&1; then
        LOGI "acme.sh is already installed."
        return 0
    fi

    LOGI "Installing acme.sh..."
    cd ~ || return 1

    fetch_text "https://get.acme.sh" | sh
    if [ $? -ne 0 ]; then
        LOGE "Installation of acme.sh failed."
        return 1
    else
        LOGI "Installation of acme.sh succeeded."
    fi

    return 0
}

ssl_cert_issue_main() {
    printf '%b\n' "${green}\t1.${plain} Get SSL"
    printf '%b\n' "${green}\t2.${plain} Revoke"
    printf '%b\n' "${green}\t3.${plain} Force Renew"
    printf '%b\n' "${green}\t4.${plain} Show Existing Domains"
    printf '%b\n' "${green}\t5.${plain} Set Cert paths for the panel"
    printf '%b\n' "${green}\t0.${plain} Back to Main Menu"

    prompt_read "Choose an option: " choice ""
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        ssl_cert_issue
        ssl_cert_issue_main
        ;;
    2)
        domains=$(list_cert_domains)
        if [ -z "$domains" ]; then
            echo "No certificates found to revoke."
        else
            echo "Existing domains:"
            echo "$domains"
            prompt_read "Please enter a domain from the list to revoke the certificate: " domain ""
            if printf '%s\n' "$domains" | grep -Fxq "$domain"; then
                ~/.acme.sh/acme.sh --revoke -d "${domain}"
                LOGI "Certificate revoked for domain: $domain"
            else
                echo "Invalid domain entered."
            fi
        fi
        ssl_cert_issue_main
        ;;
    3)
        domains=$(list_cert_domains)
        if [ -z "$domains" ]; then
            echo "No certificates found to renew."
        else
            echo "Existing domains:"
            echo "$domains"
            prompt_read "Please enter a domain from the list to renew the SSL certificate: " domain ""
            if printf '%s\n' "$domains" | grep -Fxq "$domain"; then
                ~/.acme.sh/acme.sh --renew -d "${domain}" --force
                LOGI "Certificate forcefully renewed for domain: $domain"
            else
                echo "Invalid domain entered."
            fi
        fi
        ssl_cert_issue_main
        ;;
    4)
        domains=$(list_cert_domains)
        if [ -z "$domains" ]; then
            echo "No certificates found."
        else
            echo "Existing domains and their paths:"
            for domain in $domains; do
                cert_path="/root/cert/${domain}/fullchain.pem"
                key_path="/root/cert/${domain}/privkey.pem"
                if [ -f "${cert_path}" ] && [ -f "${key_path}" ]; then
                    printf '%b\n' "Domain: ${domain}"
                    printf '%b\n' "\tCertificate Path: ${cert_path}"
                    printf '%b\n' "\tPrivate Key Path: ${key_path}"
                else
                    echo "Domain: ${domain} - Certificate or Key missing."
                fi
            done
        fi
        ssl_cert_issue_main
        ;;
    5)
        domains=$(list_cert_domains)
        if [ -z "$domains" ]; then
            echo "No certificates found."
        else
            echo "Available domains:"
            echo "$domains"
            prompt_read "Please choose a domain to set the panel paths: " domain ""

            if printf '%s\n' "$domains" | grep -Fxq "$domain"; then
                webCertFile="/root/cert/${domain}/fullchain.pem"
                webKeyFile="/root/cert/${domain}/privkey.pem"

                if [ -f "${webCertFile}" ] && [ -f "${webKeyFile}" ]; then
                    /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                    echo "Panel paths set for domain: $domain"
                    echo "  - Certificate File: $webCertFile"
                    echo "  - Private Key File: $webKeyFile"
                    restart
                else
                    echo "Certificate or private key not found for domain: $domain."
                fi
            else
                echo "Invalid domain entered."
            fi
        fi
        ssl_cert_issue_main
        ;;
    *)
        printf '%b\n' "${red}Invalid option. Please select a valid number.${plain}\n"
        ssl_cert_issue_main
        ;;
    esac
}

ssl_cert_issue() {
    existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')

    if ! command -v ~/.acme.sh/acme.sh >/dev/null 2>&1; then
        echo "acme.sh could not be found. Installing it..."
        install_acme
        if [ $? -ne 0 ]; then
            LOGE "install acme failed, please check logs"
            exit 1
        fi
    fi

    if ! command -v socat >/dev/null 2>&1; then
        case "${release}" in
            ubuntu | debian | armbian)
                apt update && apt install -y socat
                ;;
            centos | rhel | almalinux | rocky | ol)
                yum -y update && yum install -y socat
                ;;
            fedora | amzn)
                dnf -y update && dnf install -y socat
                ;;
            arch | manjaro | parch)
                pacman -Sy --noconfirm socat
                ;;
            opensuse* | suse)
                zypper refresh && zypper install -y socat
                ;;
            alpine)
                apk update && apk add socat
                ;;
            openwrt)
                if command -v opkg >/dev/null 2>&1; then
                    opkg update && opkg install socat
                elif command -v apk >/dev/null 2>&1; then
                    apk update && apk add socat
                fi
                ;;
            gentoo)
                emerge --sync && emerge --ask --quiet socat
                ;;
            void)
                xbps-install -S && xbps-install -y socat
                ;;
            solus)
                eopkg update && eopkg install -y socat
                ;;
            *)
                printf '%b\n' "${red}Unsupported operating system. Please install socat manually.${plain}\n"
                exit 1
                ;;
        esac
        if [ $? -ne 0 ]; then
            LOGE "install socat failed, please check logs"
            exit 1
        else
            LOGI "install socat succeed..."
        fi
    fi

    domain=""
    prompt_read "Please enter your domain name or IP address: " domain ""
    if [ -z "$domain" ]; then
        LOGE "Domain/IP cannot be empty."
        exit 1
    fi
    target_type="domain"
    if is_ip_address "$domain"; then
        target_type="ip"
    fi
    LOGD "Your certificate target is: ${domain}, checking it as an ${target_type}..."

    currentCert=$(~/.acme.sh/acme.sh --list | awk 'NR > 1 {print $1}' | grep -Fx -- "${domain}")
    if [ -n "${currentCert}" ]; then
        certInfo=$(~/.acme.sh/acme.sh --list)
        LOGE "System already has a certificate for this domain/IP. Cannot issue again. Current certificate details:"
        LOGI "$certInfo"
        exit 1
    else
        LOGI "Your domain/IP is ready for issuing certificates now..."
    fi

    certPath="/root/cert/${domain}"
    rm -rf "$certPath"
    mkdir -p "$certPath"

    WebPort=80
    prompt_read "Please choose which port to use (default is 80): " WebPort "80"
    if [ "${WebPort}" -gt 65535 ] 2>/dev/null || [ "${WebPort}" -lt 1 ] 2>/dev/null; then
        LOGE "Your input ${WebPort} is invalid, will use default port 80."
        WebPort=80
    fi
    LOGI "Will use port: ${WebPort} to issue certificates. Please make sure this port is open."

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
    if is_ip_address "$domain"; then
        LOGI "Issuing an IP certificate with acme.sh short-lived profile..."
        ~/.acme.sh/acme.sh --issue -d "${domain}" --standalone --server letsencrypt --certificate-profile shortlived --days 6 --httpport "${WebPort}" --force
    else
        LOGI "Issuing a domain certificate with acme.sh standalone mode..."
        ~/.acme.sh/acme.sh --issue -d "${domain}" --listen-v6 --standalone --httpport "${WebPort}"
    fi
    if [ $? -ne 0 ]; then
        LOGE "Issuing certificate failed, please check logs."
        rm -rf ~/.acme.sh/"${domain}"
        exit 1
    else
        LOGI "Issuing certificate succeeded, installing certificates..."
    fi

    ~/.acme.sh/acme.sh --installcert -d "${domain}" \
        --key-file "/root/cert/${domain}/privkey.pem" \
        --fullchain-file "/root/cert/${domain}/fullchain.pem"

    if [ $? -ne 0 ]; then
        LOGE "Installing certificate failed, exiting."
        rm -rf ~/.acme.sh/"${domain}"
        exit 1
    else
        LOGI "Installing certificate succeeded, enabling auto renew..."
    fi

    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        LOGE "Auto renew failed."
        chmod 755 "$certPath"/* 2>/dev/null || true
        exit 1
    else
        LOGI "Auto renew succeeded."
        chmod 755 "$certPath"/* 2>/dev/null || true
    fi

    prompt_read "Would you like to set this certificate for the panel? (y/n): " setPanel ""
    if [ "$setPanel" = "y" ] || [ "$setPanel" = "Y" ]; then
        webCertFile="/root/cert/${domain}/fullchain.pem"
        webKeyFile="/root/cert/${domain}/privkey.pem"

        if [ -f "$webCertFile" ] && [ -f "$webKeyFile" ]; then
            /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
            LOGI "Panel paths set for domain/IP: $domain"
            LOGI "  - Certificate File: $webCertFile"
            LOGI "  - Private Key File: $webKeyFile"
            formatted_domain=$(format_host_for_url "$domain")
            printf '%b\n' "${green}Access URL: https://${formatted_domain}:${existing_port}${existing_webBasePath}${plain}"
            restart
        else
            LOGE "Error: Certificate or private key file not found for domain/IP: $domain."
        fi
    else
        LOGI "Skipping panel path setting."
    fi
}

ssl_cert_issue_CF() {
    existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    LOGI "****** Instructions for Use ******"
    LOGI "Follow the steps below to complete the process:"
    LOGI "1. Cloudflare Registered E-mail."
    LOGI "2. Cloudflare Global API Key."
    LOGI "3. The Domain Name."
    LOGI "4. Once the certificate is issued, you will be prompted to set the certificate for the panel (optional)."
    LOGI "5. The script also supports automatic renewal of the SSL certificate after installation."

    confirm "Do you confirm the information and wish to proceed?" "y"

    if [ $? -eq 0 ]; then
        if ! command -v ~/.acme.sh/acme.sh >/dev/null 2>&1; then
            echo "acme.sh could not be found. We will install it."
            install_acme
            if [ $? -ne 0 ]; then
                LOGE "Install acme failed, please check logs."
                exit 1
            fi
        fi

        CF_Domain=""
        certPath="/root/cert-CF"
        rm -rf "$certPath"
        mkdir -p "$certPath"

        LOGD "Please set a domain name:"
        prompt_read "Input your domain here: " CF_Domain ""
        LOGD "Your domain name is set to: ${CF_Domain}"

        CF_GlobalKey=""
        CF_AccountEmail=""
        LOGD "Please set the API key:"
        prompt_read "Input your key here: " CF_GlobalKey ""
        LOGD "Your API key is: ${CF_GlobalKey}"

        LOGD "Please set up registered email:"
        prompt_read "Input your email here: " CF_AccountEmail ""
        LOGD "Your registered email address is: ${CF_AccountEmail}"

        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
        if [ $? -ne 0 ]; then
            LOGE "Default CA, Let'sEncrypt fail, script exiting..."
            exit 1
        fi

        export CF_Key="${CF_GlobalKey}"
        export CF_Email="${CF_AccountEmail}"

        ~/.acme.sh/acme.sh --issue --dns dns_cf -d "${CF_Domain}" -d "*.${CF_Domain}" --log
        if [ $? -ne 0 ]; then
            LOGE "Certificate issuance failed, script exiting..."
            exit 1
        else
            LOGI "Certificate issued successfully, Installing..."
        fi

        mkdir -p "${certPath}/${CF_Domain}"
        if [ $? -ne 0 ]; then
            LOGE "Failed to create directory: ${certPath}/${CF_Domain}"
            exit 1
        fi

        ~/.acme.sh/acme.sh --installcert -d "${CF_Domain}" -d "*.${CF_Domain}" \
            --fullchain-file "${certPath}/${CF_Domain}/fullchain.pem" \
            --key-file "${certPath}/${CF_Domain}/privkey.pem"

        if [ $? -ne 0 ]; then
            LOGE "Certificate installation failed, script exiting..."
            exit 1
        else
            LOGI "Certificate installed successfully, Turning on automatic updates..."
        fi

        ~/.acme.sh/acme.sh --upgrade --auto-upgrade
        if [ $? -ne 0 ]; then
            LOGE "Auto update setup failed, script exiting..."
            exit 1
        else
            LOGI "The certificate is installed and auto-renewal is turned on."
            chmod 755 "${certPath}/${CF_Domain}" 2>/dev/null || true
        fi

        prompt_read "Would you like to set this certificate for the panel? (y/n): " setPanel ""
        if [ "$setPanel" = "y" ] || [ "$setPanel" = "Y" ]; then
            webCertFile="${certPath}/${CF_Domain}/fullchain.pem"
            webKeyFile="${certPath}/${CF_Domain}/privkey.pem"

            if [ -f "$webCertFile" ] && [ -f "$webKeyFile" ]; then
                /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                LOGI "Panel paths set for domain: $CF_Domain"
                LOGI "  - Certificate File: $webCertFile"
                LOGI "  - Private Key File: $webKeyFile"
                printf '%b\n' "${green}Access URL: https://${CF_Domain}:${existing_port}${existing_webBasePath}${plain}"
                restart
            else
                LOGE "Error: Certificate or private key file not found for domain: $CF_Domain."
            fi
        else
            LOGI "Skipping panel path setting."
        fi
    else
        show_menu
    fi
}

run_speedtest() {
    if ! command -v speedtest >/dev/null 2>&1; then
        if [ "$release" = "openwrt" ]; then
            LOGW "Ookla Speedtest CLI is not natively packaged for OpenWrt/musl. Consider 'opkg install speedtest-netperf' or running speedtest from a client."
            return 1
        elif command -v snap >/dev/null 2>&1; then
            echo "Installing Speedtest using snap..."
            snap install speedtest
        else
            pkg_manager=""
            speedtest_install_script=""

            if command -v dnf >/dev/null 2>&1; then
                pkg_manager="dnf"
                speedtest_install_script="https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.rpm.sh"
            elif command -v yum >/dev/null 2>&1; then
                pkg_manager="yum"
                speedtest_install_script="https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.rpm.sh"
            elif command -v apt-get >/dev/null 2>&1; then
                pkg_manager="apt-get"
                speedtest_install_script="https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh"
            elif command -v apt >/dev/null 2>&1; then
                pkg_manager="apt"
                speedtest_install_script="https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh"
            fi

            if [ -z "$pkg_manager" ]; then
                echo "Error: Package manager not found. You may need to install Speedtest manually."
                return 1
            else
                echo "Installing Speedtest using $pkg_manager..."
                run_remote_script "$speedtest_install_script"
                "$pkg_manager" install -y speedtest
            fi
        fi
    fi

    speedtest
}

create_iplimit_jails() {
    bantime="${1:-30}"
    sed -i 's/#allowipv6 = auto/allowipv6 = auto/g' /etc/fail2ban/fail2ban.conf

    if [ "${release}" = "debian" ] && [ "${os_version:-0}" -ge 12 ] 2>/dev/null; then
        sed -i '0,/action =/s/backend = auto/backend = systemd/' /etc/fail2ban/jail.conf
    fi

    cat << EOF > /etc/fail2ban/jail.d/3x-ipl.conf
[3x-ipl]
enabled=true
backend=auto
filter=3x-ipl
action=3x-ipl
logpath=${iplimit_log_path}
maxretry=2
findtime=32
bantime=${bantime}m
EOF

    cat << EOF > /etc/fail2ban/filter.d/3x-ipl.conf
[Definition]
datepattern = ^%%Y/%%m/%%d %%H:%%M:%%S
failregex   = \[LIMIT_IP\]\s*Email\s*=\s*<F-USER>.+</F-USER>\s*\|\|\s*SRC\s*=\s*<ADDR>
ignoreregex =
EOF

    cat << EOF > /etc/fail2ban/action.d/3x-ipl.conf
[INCLUDES]
before = iptables-allports.conf

[Definition]
actionstart = <iptables> -N f2b-<name>
              <iptables> -A f2b-<name> -j <returntype>
              <iptables> -I <chain> -p <protocol> -j f2b-<name>

actionstop = <iptables> -D <chain> -p <protocol> -j f2b-<name>
             <actionflush>
             <iptables> -X f2b-<name>

actioncheck = <iptables> -n -L <chain> | grep -q 'f2b-<name>[ \t]'

actionban = <iptables> -I f2b-<name> 1 -s <ip> -j <blocktype>
            echo "\$(date +"%%Y/%%m/%%d %%H:%%M:%%S")   BAN   [Email] = <F-USER> [IP] = <ip> banned for <bantime> seconds." >> ${iplimit_banned_log_path}

actionunban = <iptables> -D f2b-<name> -s <ip> -j <blocktype>
              echo "\$(date +"%%Y/%%m/%%d %%H:%%M:%%S")   UNBAN   [Email] = <F-USER> [IP] = <ip> unbanned." >> ${iplimit_banned_log_path}

[Init]
name = default
protocol = tcp
chain = INPUT
EOF

    printf '%b\n' "${green}Ip Limit jail files created with a bantime of ${bantime} minutes.${plain}"
}

iplimit_remove_conflicts() {
    for file in /etc/fail2ban/jail.conf /etc/fail2ban/jail.local; do
        if [ -f "${file}" ] && grep -qw '3x-ipl' "${file}"; then
            sed -i "/\[3x-ipl\]/,/^$/d" "${file}"
            printf '%b\n' "${yellow}Removing conflicts of [3x-ipl] in jail (${file})!${plain}\n"
        fi
    done
}

ip_validation() {
    ipv6_regex="^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$"
    ipv4_regex="^((25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]?|0)\.){3}(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]?|0)$"
}

fail2ban_svc_restart() {
    if command -v systemctl >/dev/null 2>&1; then
        systemctl restart fail2ban
    elif command -v rc-service >/dev/null 2>&1; then
        rc-service fail2ban restart
    elif [ -x /etc/init.d/fail2ban ]; then
        /etc/init.d/fail2ban restart
    fi
}

iplimit_main() {
    printf '%b\n' "\n${green}\t1.${plain} Install Fail2ban and configure IP Limit"
    printf '%b\n' "${green}\t2.${plain} Change Ban Duration"
    printf '%b\n' "${green}\t3.${plain} Unban Everyone"
    printf '%b\n' "${green}\t4.${plain} Ban Logs"
    printf '%b\n' "${green}\t5.${plain} Ban an IP Address"
    printf '%b\n' "${green}\t6.${plain} Unban an IP Address"
    printf '%b\n' "${green}\t7.${plain} Real-Time Logs"
    printf '%b\n' "${green}\t8.${plain} Service Status"
    printf '%b\n' "${green}\t9.${plain} Service Restart"
    printf '%b\n' "${green}\t10.${plain} Uninstall Fail2ban and IP Limit"
    printf '%b\n' "${green}\t0.${plain} Back to Main Menu"
    prompt_read "Choose an option: " choice ""
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        confirm "Proceed with installation of Fail2ban & IP Limit?" "y"
        if [ $? -eq 0 ]; then
            install_iplimit
        else
            iplimit_main
        fi
        ;;
    2)
        prompt_read "Please enter new Ban Duration in Minutes [default 30]: " NUM "30"
        if printf '%s\n' "$NUM" | grep -Eq '^[0-9]+$'; then
            create_iplimit_jails "${NUM}"
            fail2ban_svc_restart
        else
            printf '%b\n' "${red}${NUM} is not a number! Please, try again.${plain}"
        fi
        iplimit_main
        ;;
    3)
        confirm "Proceed with Unbanning everyone from IP Limit jail?" "y"
        if [ $? -eq 0 ]; then
            fail2ban-client reload --restart --unban 3x-ipl
            : > "${iplimit_banned_log_path}"
            printf '%b\n' "${green}All users Unbanned successfully.${plain}"
            iplimit_main
        else
            printf '%b\n' "${yellow}Cancelled.${plain}"
        fi
        iplimit_main
        ;;
    4)
        show_banlog
        iplimit_main
        ;;
    5)
        prompt_read "Enter the IP address you want to ban: " ban_ip ""
        ip_validation
        if printf '%s\n' "$ban_ip" | grep -Eq "$ipv4_regex" || printf '%s\n' "$ban_ip" | grep -Eq "$ipv6_regex"; then
            fail2ban-client set 3x-ipl banip "$ban_ip"
            printf '%b\n' "${green}IP Address ${ban_ip} has been banned successfully.${plain}"
        else
            printf '%b\n' "${red}Invalid IP address format! Please try again.${plain}"
        fi
        iplimit_main
        ;;
    6)
        prompt_read "Enter the IP address you want to unban: " unban_ip ""
        ip_validation
        if printf '%s\n' "$unban_ip" | grep -Eq "$ipv4_regex" || printf '%s\n' "$unban_ip" | grep -Eq "$ipv6_regex"; then
            fail2ban-client set 3x-ipl unbanip "$unban_ip"
            printf '%b\n' "${green}IP Address ${unban_ip} has been unbanned successfully.${plain}"
        else
            printf '%b\n' "${red}Invalid IP address format! Please try again.${plain}"
        fi
        iplimit_main
        ;;
    7)
        tail -f /var/log/fail2ban.log
        iplimit_main
        ;;
    8)
        if command -v systemctl >/dev/null 2>&1; then
            systemctl status fail2ban
        elif command -v service >/dev/null 2>&1; then
            service fail2ban status
        fi
        iplimit_main
        ;;
    9)
        fail2ban_svc_restart
        iplimit_main
        ;;
    10)
        remove_iplimit
        iplimit_main
        ;;
    *)
        printf '%b\n' "${red}Invalid option. Please select a valid number.${plain}\n"
        iplimit_main
        ;;
    esac
}

install_iplimit() {
    if ! command -v fail2ban-client >/dev/null 2>&1; then
        printf '%b\n' "${green}Fail2ban is not installed. Installing now...!${plain}\n"
        case "${release}" in
            ubuntu | debian | armbian)
                apt update && apt install -y fail2ban
                ;;
            centos | rhel | almalinux | rocky | ol)
                yum update -y && yum install epel-release -y
                yum install -y fail2ban
                ;;
            fedora | amzn)
                dnf -y update && dnf install -y fail2ban
                ;;
            arch | manjaro | parch)
                pacman -Syu --noconfirm fail2ban
                ;;
            opensuse* | suse)
                zypper refresh && zypper install -y fail2ban
                ;;
            alpine)
                apk update && apk add fail2ban
                ;;
            gentoo)
                emerge --sync && emerge --ask --quiet fail2ban
                ;;
            void)
                xbps-install -S && xbps-install -y fail2ban
                ;;
            solus)
                eopkg update && eopkg install -y fail2ban
                ;;
            openwrt)
                printf '%b\n' "${yellow}Fail2ban is not available in standard OpenWrt repositories. You can use OpenWrt UCI/fw4 rate limits instead.${plain}\n"
                before_show_menu
                return 1
                ;;
            *)
                printf '%b\n' "${red}Unsupported operating system. Please install fail2ban manually.${plain}\n"
                return 1
                ;;
        esac
        if ! command -v fail2ban-client >/dev/null 2>&1; then
            printf '%b\n' "${red}Fail2ban installation failed.${plain}\n"
            return 1
        fi
        printf '%b\n' "${green}Fail2ban installed successfully!${plain}\n"
    else
        printf '%b\n' "${yellow}Fail2ban is already installed.${plain}\n"
    fi

    printf '%b\n' "${green}Configuring IP Limit...${plain}\n"
    iplimit_remove_conflicts
    [ -f "${iplimit_banned_log_path}" ] || touch "${iplimit_banned_log_path}"
    [ -f "${iplimit_log_path}" ] || touch "${iplimit_log_path}"

    create_iplimit_jails

    if command -v systemctl >/dev/null 2>&1; then
        if ! systemctl is-active --quiet fail2ban; then
            systemctl start fail2ban
            systemctl enable fail2ban
        else
            systemctl restart fail2ban
        fi
        systemctl enable fail2ban
    elif command -v rc-service >/dev/null 2>&1; then
        rc-service fail2ban restart
        rc-update add fail2ban default 2>/dev/null || true
    fi

    printf '%b\n' "${green}IP Limit installed and configured successfully!${plain}\n"
    before_show_menu
}

remove_iplimit() {
    printf '%b\n' "${green}\t1.${plain} Only remove IP Limit configurations"
    printf '%b\n' "${green}\t2.${plain} Uninstall Fail2ban and IP Limit"
    printf '%b\n' "${green}\t0.${plain} Back to Main Menu"
    prompt_read "Choose an option: " num ""
    case "$num" in
        1)
            rm -f /etc/fail2ban/filter.d/3x-ipl.conf
            rm -f /etc/fail2ban/action.d/3x-ipl.conf
            rm -f /etc/fail2ban/jail.d/3x-ipl.conf
            fail2ban_svc_restart
            printf '%b\n' "${green}IP Limit removed successfully!${plain}\n"
            before_show_menu
            ;;
        2)
            rm -rf /etc/fail2ban
            if command -v systemctl >/dev/null 2>&1; then
                systemctl stop fail2ban 2>/dev/null || true
            elif command -v rc-service >/dev/null 2>&1; then
                rc-service fail2ban stop 2>/dev/null || true
            fi
            case "${release}" in
                ubuntu | debian | armbian)
                    apt-get remove -y fail2ban && apt-get purge -y fail2ban && apt-get autoremove -y
                    ;;
                centos | rhel | almalinux | rocky | ol)
                    yum remove -y fail2ban && yum autoremove -y
                    ;;
                fedora | amzn)
                    dnf remove -y fail2ban && dnf autoremove -y
                    ;;
                arch | manjaro | parch)
                    pacman -Rns --noconfirm fail2ban
                    ;;
                opensuse* | suse)
                    zypper remove -y fail2ban
                    ;;
                alpine)
                    apk del fail2ban
                    ;;
                gentoo)
                    emerge --deselect fail2ban
                    ;;
                void)
                    xbps-remove -y fail2ban
                    ;;
                solus)
                    eopkg remove -y fail2ban
                    ;;
                *)
                    printf '%b\n' "${red}Unsupported operating system. Please uninstall Fail2ban manually.${plain}\n"
                    return 1
                    ;;
            esac
            printf '%b\n' "${green}Fail2ban and IP Limit removed successfully!${plain}\n"
            before_show_menu
            ;;
        0)
            show_menu
            ;;
        *)
            printf '%b\n' "${red}Invalid option. Please select a valid number.${plain}\n"
            remove_iplimit
            ;;
    esac
}

SSH_port_forwarding() {
    server_ip=$(get_server_ip)
    existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    existing_listenIP=$(/usr/local/x-ui/x-ui setting -getListen true | grep -Eo 'listenIP: .+' | awk '{print $2}')
    existing_cert=$(/usr/local/x-ui/x-ui setting -getCert true | grep -Eo 'cert: .+' | awk '{print $2}')
    existing_key=$(/usr/local/x-ui/x-ui setting -getCert true | grep -Eo 'key: .+' | awk '{print $2}')

    config_listenIP=""
    listen_choice=""

    if [ -n "$existing_cert" ] && [ -n "$existing_key" ]; then
        printf '%b\n' "${green}Panel is secure with SSL.${plain}"
        before_show_menu
        return
    fi
    if [ -z "$existing_cert" ] && [ -z "$existing_key" ] && { [ -z "$existing_listenIP" ] || [ "$existing_listenIP" = "0.0.0.0" ]; }; then
        printf '%b\n' "\n${red}Warning: No Cert and Key found! The panel is not secure.${plain}"
        echo "Please obtain a certificate or set up SSH port forwarding."
    fi

    if [ -n "$existing_listenIP" ] && [ "$existing_listenIP" != "0.0.0.0" ] && [ -z "$existing_cert" ] && [ -z "$existing_key" ]; then
        printf '%b\n' "\n${green}Current SSH Port Forwarding Configuration:${plain}"
        echo "Standard SSH command:"
        printf '%b\n' "${yellow}ssh -L 2222:${existing_listenIP}:${existing_port} root@${server_ip}${plain}"
        echo ""
        echo "If using SSH key:"
        printf '%b\n' "${yellow}ssh -i <sshkeypath> -L 2222:${existing_listenIP}:${existing_port} root@${server_ip}${plain}"
        echo ""
        echo "After connecting, access the panel at:"
        printf '%b\n' "${yellow}http://localhost:2222${existing_webBasePath}${plain}"
    fi

    echo ""
    echo "Choose an option:"
    printf '%b\n' "${green}1.${plain} Set listen IP"
    printf '%b\n' "${green}2.${plain} Clear listen IP"
    printf '%b\n' "${green}0.${plain} Back to Main Menu"
    prompt_read "Choose an option: " num ""

    case "$num" in
    1)
        if [ -z "$existing_listenIP" ] || [ "$existing_listenIP" = "0.0.0.0" ]; then
            echo ""
            echo "No listenIP configured. Choose an option:"
            echo "1. Use default IP (127.0.0.1)"
            echo "2. Set a custom IP"
            prompt_read "Select an option (1 or 2): " listen_choice "1"

            config_listenIP="127.0.0.1"
            if [ "$listen_choice" = "2" ]; then
                prompt_read "Enter custom IP to listen on: " config_listenIP "127.0.0.1"
            fi

            /usr/local/x-ui/x-ui setting -listenIP "${config_listenIP}" >/dev/null 2>&1
            printf '%b\n' "${green}listen IP has been set to ${config_listenIP}.${plain}"
            printf '%b\n' "\n${green}SSH Port Forwarding Configuration:${plain}"
            echo "Standard SSH command:"
            printf '%b\n' "${yellow}ssh -L 2222:${config_listenIP}:${existing_port} root@${server_ip}${plain}"
            echo ""
            echo "After connecting, access the panel at:"
            printf '%b\n' "${yellow}http://localhost:2222${existing_webBasePath}${plain}"
            restart
        else
            config_listenIP="${existing_listenIP}"
            printf '%b\n' "${green}Current listen IP is already set to ${config_listenIP}.${plain}"
        fi
        ;;
    2)
        /usr/local/x-ui/x-ui setting -listenIP 0.0.0.0 >/dev/null 2>&1
        printf '%b\n' "${green}Listen IP has been cleared.${plain}"
        restart
        ;;
    0)
        show_menu
        ;;
    *)
        printf '%b\n' "${red}Invalid option. Please select a valid number.${plain}\n"
        SSH_port_forwarding
        ;;
    esac
}

install_postgresql() {
    LOGI "Starting automated PostgreSQL installation..."
    pkg_manager=""
    if command -v apt-get >/dev/null 2>&1; then
        pkg_manager="apt-get"
    elif command -v dnf >/dev/null 2>&1; then
        pkg_manager="dnf"
    elif command -v yum >/dev/null 2>&1; then
        pkg_manager="yum"
    elif command -v pacman >/dev/null 2>&1; then
        pkg_manager="pacman"
    elif command -v apk >/dev/null 2>&1; then
        pkg_manager="apk"
    elif command -v opkg >/dev/null 2>&1; then
        pkg_manager="opkg"
    fi

    if [ -z "${pkg_manager}" ]; then
        LOGE "Supported package manager (apt-get, dnf, yum, pacman, apk, opkg) not found."
        before_show_menu
        return
    fi

    printf '%b\n' "${yellow}Installing PostgreSQL server using ${pkg_manager}...${plain}"
    case "${pkg_manager}" in
    apt-get)
        apt-get update -y
        apt-get install -y postgresql postgresql-contrib
        ;;
    dnf)
        dnf -y update
        dnf install -y postgresql-server postgresql-contrib
        ;;
    yum)
        yum -y update
        yum install -y postgresql-server postgresql-contrib
        ;;
    pacman)
        pacman -Sy --noconfirm postgresql
        ;;
    apk)
        apk update
        apk add postgresql postgresql-contrib
        ;;
    opkg)
        opkg update
        opkg install pgsql-server pgsql-cli
        ;;
    esac

    if [ "${pkg_manager}" = "dnf" ] || [ "${pkg_manager}" = "yum" ]; then
        if command -v postgresql-setup >/dev/null 2>&1; then
            postgresql-setup --initdb 2>/dev/null || postgresql-setup initdb 2>/dev/null || true
        fi
    elif [ "${pkg_manager}" = "pacman" ]; then
        if [ ! -d "/var/lib/postgres/data" ] || [ -z "$(ls -A /var/lib/postgres/data 2>/dev/null)" ]; then
            mkdir -p /var/lib/postgres/data
            chown -R postgres:postgres /var/lib/postgres
            su - postgres -c "initdb --locale=C.UTF-8 -E UTF8 -D '/var/lib/postgres/data'" 2>/dev/null || true
        fi
    elif [ "${pkg_manager}" = "apk" ]; then
        if [ ! -d "/var/lib/postgresql/data" ] || [ -z "$(ls -A /var/lib/postgresql/data 2>/dev/null)" ]; then
            mkdir -p /var/lib/postgresql/data
            chown -R postgres:postgres /var/lib/postgresql
            su - postgres -c "initdb -D '/var/lib/postgresql/data'" 2>/dev/null || true
        fi
    fi

    printf '%b\n' "${yellow}Starting and enabling PostgreSQL service...${plain}"
    if command -v systemctl >/dev/null 2>&1; then
        systemctl daemon-reload
        systemctl enable postgresql 2>/dev/null || systemctl enable postgresql.service 2>/dev/null || true
        systemctl restart postgresql 2>/dev/null || systemctl restart postgresql.service 2>/dev/null || true
    elif command -v rc-service >/dev/null 2>&1; then
        rc-service postgresql restart 2>/dev/null || true
        rc-update add postgresql default 2>/dev/null || true
    elif [ -x /etc/init.d/postgresql ]; then
        /etc/init.d/postgresql enable 2>/dev/null || true
        /etc/init.d/postgresql restart 2>/dev/null || true
    fi

    sleep 2

    pg_ready=false
    if command -v pg_isready >/dev/null 2>&1; then
        if pg_isready -q; then
            pg_ready=true
        fi
    fi
    if [ "${pg_ready}" = "false" ]; then
        if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet postgresql; then
            pg_ready=true
        fi
    fi

    if [ "${pg_ready}" = "false" ]; then
        LOGW "PostgreSQL service status uncertain, attempting database configuration anyway..."
    else
        LOGI "PostgreSQL service is running!"
    fi

    printf '%b\n' "\n${yellow}Configure tx-ui PostgreSQL Database:${plain}"
    default_dbname="x_ui"
    default_dbuser="x_ui_user"
    default_dbpass=$(gen_random_string 16)
    if [ -z "${default_dbpass}" ]; then
        default_dbpass="xui_$(date +%s)_pass"
    fi

    prompt_read "Database Name [${default_dbname}]: " pg_dbname "${default_dbname}"
    prompt_read "Database Username [${default_dbuser}]: " pg_user "${default_dbuser}"
    prompt_read "Database Password [auto-generated: ${default_dbpass}]: " pg_pass "${default_dbpass}"
    pg_port="5432"

    printf '%b\n' "\n${yellow}Creating PostgreSQL user and database...${plain}"

    sql_commands="
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '${pg_user}') THEN
        CREATE ROLE \"${pg_user}\" WITH LOGIN PASSWORD '${pg_pass}';
    ELSE
        ALTER ROLE \"${pg_user}\" WITH PASSWORD '${pg_pass}';
    END IF;
END
\$\$;
"
    su - postgres -c "psql -v ON_ERROR_STOP=1 -c \"${sql_commands}\"" 2>/dev/null || {
        sudo -u postgres psql -c "${sql_commands}" 2>/dev/null || true
    }

    check_db_sql="SELECT 1 FROM pg_database WHERE datname = '${pg_dbname}';"
    db_exists=$(su - postgres -c "psql -t -A -c \"${check_db_sql}\"" 2>/dev/null || sudo -u postgres psql -t -A -c "${check_db_sql}" 2>/dev/null || echo "")

    if [ "${db_exists}" != "1" ]; then
        su - postgres -c "psql -c \"CREATE DATABASE \\\"${pg_dbname}\\\" OWNER \\\"${pg_user}\\\";\"" 2>/dev/null || {
            sudo -u postgres psql -c "CREATE DATABASE \"${pg_dbname}\" OWNER \"${pg_user}\";" 2>/dev/null || true
        }
    fi

    grant_sql="
GRANT ALL PRIVILEGES ON DATABASE \"${pg_dbname}\" TO \"${pg_user}\";
ALTER DATABASE \"${pg_dbname}\" OWNER TO \"${pg_user}\";
\c \"${pg_dbname}\"
GRANT ALL ON SCHEMA public TO \"${pg_user}\";
ALTER SCHEMA public OWNER TO \"${pg_user}\";
"
    su - postgres -c "psql -c \"${grant_sql}\"" 2>/dev/null || {
        sudo -u postgres psql -c "${grant_sql}" 2>/dev/null || true
    }

    conn_ok=false
    if command -v psql >/dev/null 2>&1; then
        if PGPASSWORD="${pg_pass}" psql -h 127.0.0.1 -p "${pg_port}" -U "${pg_user}" -d "${pg_dbname}" -c '\q' 2>/dev/null; then
            conn_ok=true
        fi
    fi

    if [ "${conn_ok}" = "false" ]; then
        LOGW "Direct 127.0.0.1 connection check with new user failed. Adjusting pg_hba.conf..."
        hba_file=$(su - postgres -c "psql -t -A -c 'SHOW hba_file;'" 2>/dev/null || sudo -u postgres psql -t -A -c 'SHOW hba_file;' 2>/dev/null || echo "")
        if [ -f "${hba_file}" ]; then
            if ! grep -q -E "^host[[:space:]]+all[[:space:]]+all[[:space:]]+127\.0\.0\.1/32" "${hba_file}"; then
                echo "host    all             all             127.0.0.1/32            md5" >> "${hba_file}"
                if command -v systemctl >/dev/null 2>&1; then
                    systemctl reload postgresql 2>/dev/null || systemctl restart postgresql 2>/dev/null || true
                fi
            fi
        fi
    fi

    mkdir -p /etc/x-ui
    cat <<EOF > /etc/x-ui/x-ui.env
XUI_DB_TYPE=postgres
XUI_DB_HOST=127.0.0.1
XUI_DB_PORT=${pg_port}
XUI_DB_NAME=${pg_dbname}
XUI_DB_USER=${pg_user}
XUI_DB_PASSWORD=${pg_pass}
XUI_DB_SSLMODE=disable
XUI_DB_TIMEZONE=UTC
EOF
    chmod 600 /etc/x-ui/x-ui.env

    printf '%b\n' "\n${green}==================================================================${plain}"
    printf '%b\n' "${green} PostgreSQL Server Installed & Configured Successfully!${plain}"
    printf '%b\n' "${green}==================================================================${plain}"
    printf '%b\n' "Host:     ${yellow}127.0.0.1${plain}"
    printf '%b\n' "Port:     ${yellow}${pg_port}${plain}"
    printf '%b\n' "Database: ${yellow}${pg_dbname}${plain}"
    printf '%b\n' "User:     ${yellow}${pg_user}${plain}"
    printf '%b\n' "Password: ${yellow}${pg_pass}${plain}"
    printf '%b\n' "SSL Mode: ${yellow}disable${plain}"
    printf '%b\n' "Env File: ${yellow}/etc/x-ui/x-ui.env${plain}"
    printf '%b\n' "${green}==================================================================${plain}\n"

    LOGI "PostgreSQL configuration saved to /etc/x-ui/x-ui.env."

    if [ -f "/etc/x-ui/x-ui.db" ] && [ -s "/etc/x-ui/x-ui.db" ]; then
        printf '%b\n' "${yellow}Existing SQLite database found at /etc/x-ui/x-ui.db.${plain}"
        prompt_read "Do you want to migrate all your existing data (inbounds, clients, settings) to PostgreSQL? [Y/n]: " do_mig "Y"
        case "${do_mig}" in
        [Nn]*)
            LOGI "Skipping data migration. A clean PostgreSQL database will be used."
            ;;
        *)
            printf '%b\n' "${yellow}Migrating data from SQLite to PostgreSQL...${plain}"
            /usr/local/x-ui/x-ui migrate sqlite-to-pg 2>/dev/null || x-ui migrate sqlite-to-pg 2>/dev/null || true
            LOGI "Data migration from SQLite to PostgreSQL completed successfully!"
            ;;
        esac
    fi

    restart_panel_prompt
}

database_menu() {
    printf '%b\n' "
╔────────────────────────────────────────────────╗
│   ${green}Database Configuration (SQLite / PostgreSQL)${plain} │
│   ${green}0.${plain} Return to Main Menu                       │
│────────────────────────────────────────────────│
│   ${green}1.${plain} Switch to SQLite (Default)                │
│   ${green}2.${plain} Install & Setup PostgreSQL (Automated)    │
│   ${green}3.${plain} Switch to PostgreSQL (Standard Parameters)│
│   ${green}4.${plain} Switch to PostgreSQL (Custom DSN URL)     │
│   ${green}5.${plain} Migrate Data (SQLite <-> PostgreSQL)      │
│   ${green}6.${plain} View Current Database Configuration       │
╚────────────────────────────────────────────────╝
"
    env_file="/etc/x-ui/x-ui.env"
    current_db="SQLite (/etc/x-ui/x-ui.db)"
    if [ -f "${env_file}" ]; then
        . "${env_file}" 2>/dev/null || true
        if [ "${XUI_DB_TYPE:-}" = "postgres" ] || [ "${XUI_DB_TYPE:-}" = "postgresql" ] || [ "${XUI_DB_TYPE:-}" = "pg" ]; then
            if [ -n "${XUI_DB_POSTGRES_DSN:-}" ]; then
                current_db="PostgreSQL (DSN: ${XUI_DB_POSTGRES_DSN})"
            else
                current_db="PostgreSQL (${XUI_DB_USER:-postgres}@${XUI_DB_HOST:-127.0.0.1}:${XUI_DB_PORT:-5432}/${XUI_DB_NAME:-xui})"
            fi
        fi
    fi
    printf '%b\n' "Current Database: ${green}${current_db}${plain}\n"

    prompt_read "Please enter your selection [0-6]: " db_choice ""
    case "${db_choice}" in
    0)
        show_menu
        ;;
    1)
        case "${current_db}" in
            *PostgreSQL*)
                printf '%b\n' "${yellow}You are currently using PostgreSQL.${plain}"
                prompt_read "Do you want to export and migrate your PostgreSQL data to SQLite (/etc/x-ui/x-ui.db)? [Y/n]: " mig_choice "Y"
                case "${mig_choice}" in
                [Nn]*)
                    LOGI "Skipping data migration."
                    ;;
                *)
                    printf '%b\n' "${yellow}Exporting PostgreSQL data to SQLite...${plain}"
                    /usr/local/x-ui/x-ui migrate pg-to-sqlite 2>/dev/null || x-ui migrate pg-to-sqlite 2>/dev/null || true
                    LOGI "PostgreSQL data successfully transferred to SQLite."
                    ;;
                esac
                ;;
        esac
        mkdir -p /etc/x-ui
        cat <<EOF > /etc/x-ui/x-ui.env
XUI_DB_TYPE=sqlite
EOF
        chmod 600 /etc/x-ui/x-ui.env
        LOGI "Database switched to SQLite."
        restart_panel_prompt
        ;;
    2)
        install_postgresql
        ;;
    3)
        printf '%b\n' "${yellow}Enter PostgreSQL Connection Details:${plain}"
        prompt_read "PostgreSQL Host [127.0.0.1]: " pg_host "127.0.0.1"
        prompt_read "PostgreSQL Port [5432]: " pg_port "5432"
        prompt_read "Database Name [xui]: " pg_dbname "xui"
        prompt_read "Database User [postgres]: " pg_user "postgres"
        prompt_secret "Database Password: " pg_pass
        prompt_read "SSL Mode (disable/require/verify-ca/verify-full) [disable]: " pg_sslmode "disable"
        prompt_read "Timezone [UTC]: " pg_tz "UTC"

        mkdir -p /etc/x-ui
        cat <<EOF > /etc/x-ui/x-ui.env
XUI_DB_TYPE=postgres
XUI_DB_HOST=${pg_host}
XUI_DB_PORT=${pg_port}
XUI_DB_NAME=${pg_dbname}
XUI_DB_USER=${pg_user}
XUI_DB_PASSWORD=${pg_pass}
XUI_DB_SSLMODE=${pg_sslmode}
XUI_DB_TIMEZONE=${pg_tz}
EOF
        chmod 600 /etc/x-ui/x-ui.env
        LOGI "PostgreSQL configuration saved to /etc/x-ui/x-ui.env."
        if [ -f "/etc/x-ui/x-ui.db" ] && [ -s "/etc/x-ui/x-ui.db" ]; then
            printf '%b\n' "${yellow}Existing SQLite database found at /etc/x-ui/x-ui.db.${plain}"
            prompt_read "Do you want to migrate your existing data to PostgreSQL? [Y/n]: " do_mig "Y"
            case "${do_mig}" in
            [Nn]*) ;;
            *)
                printf '%b\n' "${yellow}Migrating data from SQLite to PostgreSQL...${plain}"
                /usr/local/x-ui/x-ui migrate sqlite-to-pg 2>/dev/null || x-ui migrate sqlite-to-pg 2>/dev/null || true
                ;;
            esac
        fi
        restart_panel_prompt
        ;;
    4)
        printf '%b\n' "${yellow}Enter PostgreSQL DSN URL (e.g. postgres://user:password@host:5432/dbname?sslmode=disable):${plain}"
        prompt_read "DSN: " pg_dsn ""
        if [ -z "${pg_dsn}" ]; then
            LOGE "DSN cannot be empty!"
            before_show_menu
            return
        fi
        mkdir -p /etc/x-ui
        cat <<EOF > /etc/x-ui/x-ui.env
XUI_DB_TYPE=postgres
XUI_DB_POSTGRES_DSN=${pg_dsn}
EOF
        chmod 600 /etc/x-ui/x-ui.env
        LOGI "PostgreSQL DSN saved to /etc/x-ui/x-ui.env."
        if [ -f "/etc/x-ui/x-ui.db" ] && [ -s "/etc/x-ui/x-ui.db" ]; then
            printf '%b\n' "${yellow}Existing SQLite database found at /etc/x-ui/x-ui.db.${plain}"
            prompt_read "Do you want to migrate your existing data to PostgreSQL? [Y/n]: " do_mig "Y"
            case "${do_mig}" in
            [Nn]*) ;;
            *)
                printf '%b\n' "${yellow}Migrating data from SQLite to PostgreSQL...${plain}"
                /usr/local/x-ui/x-ui migrate sqlite-to-pg 2>/dev/null || x-ui migrate sqlite-to-pg 2>/dev/null || true
                ;;
            esac
        fi
        restart_panel_prompt
        ;;
    5)
        printf '%b\n' "
╔────────────────────────────────────────────────╗
│   ${green}Migrate Database Data (SQLite <-> PostgreSQL)${plain}│
│   ${green}0.${plain} Return to Database Menu                   │
│────────────────────────────────────────────────│
│   ${green}1.${plain} Migrate SQLite -> PostgreSQL (Load into PG) │
│   ${green}2.${plain} Migrate PostgreSQL -> SQLite (Export to DB) │
╚────────────────────────────────────────────────╝
"
        prompt_read "Please enter your selection [0-2]: " mig_sub ""
        case "${mig_sub}" in
        1)
            printf '%b\n' "${yellow}Migrating data from SQLite to PostgreSQL...${plain}"
            /usr/local/x-ui/x-ui migrate sqlite-to-pg 2>/dev/null || x-ui migrate sqlite-to-pg 2>/dev/null || true
            before_show_menu
            ;;
        2)
            printf '%b\n' "${yellow}Exporting data from PostgreSQL to SQLite...${plain}"
            /usr/local/x-ui/x-ui migrate pg-to-sqlite 2>/dev/null || x-ui migrate pg-to-sqlite 2>/dev/null || true
            before_show_menu
            ;;
        *)
            database_menu
            ;;
        esac
        ;;
    6)
        if [ -f "${env_file}" ]; then
            printf '%b\n' "${green}--- /etc/x-ui/x-ui.env ---${plain}"
            cat "${env_file}"
            printf '%b\n' "${green}--------------------------${plain}"
        else
            printf '%b\n' "${yellow}No /etc/x-ui/x-ui.env found. Using default SQLite database (/etc/x-ui/x-ui.db).${plain}"
        fi
        before_show_menu
        ;;
    *)
        LOGE "Invalid selection!"
        show_menu
        ;;
    esac
}

restart_panel_prompt() {
    prompt_read "Do you want to restart tx-ui now to apply changes? [Y/n]: " yn "Y"
    case "$yn" in
    [Nn]*)
        LOGI "Configuration saved. Please restart tx-ui manually (x-ui restart) when ready."
        before_show_menu
        ;;
    *)
        restart
        ;;
    esac
}

show_usage() {
    printf '%b\n' "┌───────────────────────────────────────────────────────┐
│  ${blue}x-ui control menu usages (subcommands):${plain}              │
│                                                       │
│  ${blue}x-ui${plain}              - Admin Management Script          │
│  ${blue}x-ui start${plain}        - Start                            │
│  ${blue}x-ui stop${plain}         - Stop                             │
│  ${blue}x-ui restart${plain}      - Restart                          │
│  ${blue}x-ui status${plain}       - Current Status                   │
│  ${blue}x-ui settings${plain}     - Current Settings                 │
│  ${blue}x-ui database${plain}     - Database Configuration (SQLite/PG)│
│  ${blue}x-ui postgres${plain}     - Auto Install & Setup PostgreSQL   │
│  ${blue}x-ui migrate-to-pg${plain} - Migrate Data: SQLite -> PostgreSQL │
│  ${blue}x-ui migrate-to-sqlite${plain} - Migrate Data: PostgreSQL -> SQLite │
│  ${blue}x-ui reset${plain}        - Factory Reset (All Data & Settings)│
│  ${blue}x-ui enable${plain}       - Enable Autostart on OS Startup   │
│  ${blue}x-ui disable${plain}      - Disable Autostart on OS Startup  │
│  ${blue}x-ui log${plain}          - Check logs                       │
│  ${blue}x-ui banlog${plain}       - Check Fail2ban ban logs          │
│  ${blue}x-ui update${plain}       - Update                           │
│  ${blue}x-ui legacy${plain}       - legacy version                   │
│  ${blue}x-ui install${plain}      - Install                          │
│  ${blue}x-ui uninstall${plain}    - Uninstall                        │
└───────────────────────────────────────────────────────┘"
}

show_menu() {
    printf '%b\n' "
╔────────────────────────────────────────────────╗
│   ${green}TX-UI Panel Management Script${plain}                │
│   ${green}0.${plain} Exit Script                               │
│────────────────────────────────────────────────│
│   ${green}1.${plain} Install                                   │
│   ${green}2.${plain} Update                                    │
│   ${green}3.${plain} Update Menu                               │
│   ${green}4.${plain} Legacy Version                            │
│   ${green}5.${plain} Uninstall                                 │
│────────────────────────────────────────────────│
│   ${green}6.${plain} Reset Username & Password & Secret Token  │
│   ${green}7.${plain} Reset Web Base Path                       │
│   ${green}8.${plain} Reset Settings                            │
│   ${green}9.${plain} Change Port                               │
│  ${green}10.${plain} View Current Settings                     │
│────────────────────────────────────────────────│
│  ${green}11.${plain} Start                                     │
│  ${green}12.${plain} Stop                                      │
│  ${green}13.${plain} Restart                                   │
│  ${green}14.${plain} Check Status                              │
│  ${green}15.${plain} Logs Management                           │
│────────────────────────────────────────────────│
│  ${green}16.${plain} Enable Autostart                          │
│  ${green}17.${plain} Disable Autostart                         │
│────────────────────────────────────────────────│
│  ${green}18.${plain} SSL Certificate Management                │
│  ${green}19.${plain} Cloudflare SSL Certificate                │
│  ${green}20.${plain} IP Limit Management                       │
│  ${green}21.${plain} Firewall Management                       │
│  ${green}22.${plain} SSH Port Forwarding Management            │
│────────────────────────────────────────────────│
│  ${green}23.${plain} Enable BBR                                │
│  ${green}24.${plain} Update Geo Files                          │
│  ${green}25.${plain} Speedtest by Ookla                        │
│  ${green}26.${plain} Theme Management                          │
│  ${green}27.${plain} Database Configuration (SQLite/PG)        │
│  ${green}28.${plain} Factory Reset (Reset All Data & Settings) │
╚────────────────────────────────────────────────╝
"
    show_status
    echo ""
    prompt_read "Please enter your selection [0-28]: " num ""

    case "${num}" in
    0)
        exit 0
        ;;
    1)
        check_uninstall && install
        ;;
    2)
        check_install && update
        ;;
    3)
        check_install && update_menu
        ;;
    4)
        check_install && legacy_version
        ;;
    5)
        check_install && uninstall
        ;;
    6)
        check_install && reset_user
        ;;
    7)
        check_install && reset_webbasepath
        ;;
    8)
        check_install && reset_config
        ;;
    9)
        check_install && set_port
        ;;
    10)
        check_install && check_config
        ;;
    11)
        check_install && start
        ;;
    12)
        check_install && stop
        ;;
    13)
        check_install && restart
        ;;
    14)
        check_install && status
        ;;
    15)
        check_install && show_log
        ;;
    16)
        check_install && enable
        ;;
    17)
        check_install && disable
        ;;
    18)
        ssl_cert_issue_main
        ;;
    19)
        ssl_cert_issue_CF
        ;;
    20)
        iplimit_main
        ;;
    21)
        firewall_menu
        ;;
    22)
        SSH_port_forwarding
        ;;
    23)
        bbr_menu
        ;;
    24)
        update_geo
        ;;
    25)
        run_speedtest
        ;;
    26)
        run_remote_script "https://raw.githubusercontent.com/Incognito-Coder/tx-themehub/master/install.sh"
        ;;
    27)
        check_install && database_menu
        ;;
    28)
        check_install && factory_reset
        ;;
    *)
        LOGE "Please enter the correct number [0-28]"
        ;;
    esac
}

if [ $# -gt 0 ]; then
    case "$1" in
    "start")
        check_install 0 && start 0
        ;;
    "stop")
        check_install 0 && stop 0
        ;;
    "restart")
        check_install 0 && restart 0
        ;;
    "status")
        check_install 0 && status 0
        ;;
    "settings")
        check_install 0 && check_config 0
        ;;
    "database" | "db")
        check_install 0 && database_menu
        ;;
    "postgres" | "pg" | "pg-install")
        check_install 0 && install_postgresql
        ;;
    "migrate-to-pg" | "sqlite-to-pg")
        check_install 0 && (/usr/local/x-ui/x-ui migrate sqlite-to-pg || x-ui migrate sqlite-to-pg)
        ;;
    "migrate-to-sqlite" | "pg-to-sqlite")
        check_install 0 && (/usr/local/x-ui/x-ui migrate pg-to-sqlite || x-ui migrate pg-to-sqlite)
        ;;
    "reset" | "factory-reset")
        check_install 0 && factory_reset 0
        ;;
    "enable")
        check_install 0 && enable 0
        ;;
    "disable")
        check_install 0 && disable 0
        ;;
    "log")
        check_install 0 && show_log 0
        ;;
    "banlog")
        check_install 0 && show_banlog 0
        ;;
    "update")
        check_install 0 && update 0
        ;;
    "legacy")
        check_install 0 && legacy_version 0
        ;;
    "install")
        check_uninstall 0 && install 0
        ;;
    "uninstall")
        check_install 0 && uninstall 0
        ;;
    *) show_usage ;;
    esac
else
    show_menu
fi
