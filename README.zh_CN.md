[English](/README.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md)

<p align="center">
  <picture>
    <img alt="tx-ui" src="./media/tx-ui-dark.png" style="width:512px;height:512px;">
  </picture>
</p>

**一个更好的面板 • 基于Xray Core构建**
**该项目是 3x-ui 面板的一个分支。**

[![](https://img.shields.io/github/v/release/Incognito-Coder/tx-ui.svg)](https://github.com/Incognito-Coder/tx-ui/releases)
[![](https://img.shields.io/github/actions/workflow/status/Incognito-Coder/tx-ui/release.yml.svg)](#)
[![GO Version](https://img.shields.io/github/go-mod/go-version/Incognito-Coder/tx-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/Incognito-Coder/tx-ui/total.svg)](#)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

> **Disclaimer:** 此项目仅供个人学习交流，请不要用于非法目的，请不要在生产环境中使用。

**如果此项目对你有用，请给一个**:star2:

**USDT(TON):** ```UQD6dwK0jcZuJfH-xMPu4HcnXru9xMKVIzTy_2oZZm-HZzCl```

## 安装 & 升级

```
bash <(curl -Ls https://raw.githubusercontent.com/Incognito-Coder/tx-ui/master/install.sh)
```

### SSL证书

<details>
  <summary>点击查看SSL证书详情</summary>

### ACME

使用ACME管理SSL证书：

1. 确保您的域名正确解析到服务器。
2. 在终端中运行 `x-ui` 命令，然后选择 `SSL证书管理`。
3. 您将看到以下选项：

    - **Get SSL:** 获取SSL证书。
    - **Revoke:** 吊销现有的SSL证书。
    - **Force Renew:** 强制更新SSL证书。
    - **Show Existing Domains:** 显示服务器上所有可用的域证书。
    - **Set Certificate Paths for the Panel:** 指定用于面板的域证书。

### Certbot

安装并使用Certbot：

```sh
apt-get install certbot -y
certbot certonly --standalone --agree-tos --register-unsafely-without-email -d yourdomain.com
certbot renew --dry-run
```

### Cloudflare

管理脚本内置了Cloudflare的SSL证书申请。要使用此脚本申请证书，您需要以下信息：

- Cloudflare注册的电子邮件
- Cloudflare全局API密钥
- 域名必须通过Cloudflare解析到当前服务器

**如何获取Cloudflare全局API密钥：**

1. 在终端中运行 `x-ui` 命令，然后选择 `Cloudflare SSL证书`。
2. 访问链接：[Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens)。
3. 点击“查看全局API密钥”（参见下图）：
   ![](media/APIKey1.PNG)
4. 您可能需要重新验证您的账户。之后将显示API密钥（参见下图）：
   ![](media/APIKey2.png)

使用时，只需输入您的 `域名`、`电子邮件` 和 `API密钥`。如下图所示：
![](media/DetailEnter.png)

</details>

## 手动安装 & 升级

<details>
  <summary>点击查看 手动安装 & 升级</summary>

#### 使用

1. 若要将最新版本的压缩包直接下载到服务器，请运行以下命令：

```sh
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;; 
  i*86 | x86) XUI_ARCH="386" ;; 
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;; 
  armv7* | armv7) XUI_ARCH="armv7" ;; 
  armv6* | armv6) XUI_ARCH="armv6" ;; 
  armv5* | armv5) XUI_ARCH="armv5" ;; 
  *) XUI_ARCH="amd64" ;; 
esac


wget https://github.com/Incognito-Coder/tx-ui/releases/latest/download/x-ui-linux-${XUI_ARCH}.tar.gz
```

2. 下载压缩包后，执行以下命令安装或升级 x-ui：

```sh
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;; 
  i*86 | x86) XUI_ARCH="386" ;; 
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;; 
  armv7* | armv7) XUI_ARCH="armv7" ;; 
  armv6* | armv6) XUI_ARCH="armv6" ;; 
  armv5* | armv5) XUI_ARCH="armv5" ;; 
  *) XUI_ARCH="amd64" ;; 
esac

cd /root/ 
rm -rf x-ui/ /usr/local/x-ui/ /usr/bin/x-ui
tar zxvf x-ui-linux-${XUI_ARCH}.tar.gz
chmod +x x-ui/x-ui x-ui/bin/xray-linux-* x-ui/x-ui.sh
cp x-ui/x-ui.sh /usr/bin/x-ui
cp -f x-ui/x-ui.service /etc/systemd/system/
mv x-ui/ /usr/local/
systemctl daemon-reload
systemctl enable x-ui
systemctl restart x-ui
```

</details>

## 通过Docker安装

<details>
  <summary>点击查看 通过Docker安装</summary>

#### 使用

1. 安装Docker：

   ```sh
   bash <(curl -sSL https://get.docker.com)
   ```

2. 克隆仓库：

   ```sh
   git clone https://github.com/Incognito-Coder/tx-ui.git
   cd tx-ui
   ```

3. 运行服务：

   ```sh
   docker compose up -d
   ```

   或

   ```sh
   docker run -itd \
      -e XRAY_VMESS_AEAD_FORCED=false \
      -v $PWD/db/:/etc/x-ui/ \
      -v $PWD/cert/:/root/cert/ \
      --network=host \
      --restart=unless-stopped \
      --name tx-ui \
      ghcr.io/incognitocoder/tx-ui:latest
   ```

更新至最新版本

   ```sh
    cd tx-ui
    docker compose down
    docker compose pull tx-ui
    docker compose up -d
   ```

从Docker中删除tx-ui

   ```sh
    docker stop tx-ui
    docker rm tx-ui
    cd --
    rm -r tx-ui
   ```

</details>

## Nginx 设置

<details>
  <summary>点击查看 反向代理配置</summary>

#### Nginx反向代理

```nginx
location / {
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Range $http_range;
    proxy_set_header If-Range $http_if_range; 
    proxy_redirect off;
    proxy_pass http://127.0.0.1:2053;
}
```

#### Nginx子路径

- 确保 `/sub` 面板设置中的"面板url根路径"一致
- 面板设置中的 `url` 需要以 `/` 结尾

```nginx
location /sub {
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Range $http_range;
    proxy_set_header If-Range $http_if_range; 
    proxy_redirect off;
    proxy_pass http://127.0.0.1:2053;
}
```

</details>

## 建议使用的操作系统

- Ubuntu 22.04+
- Debian 12+
- CentOS 8+
- OpenEuler 22.03+
- Fedora 36+
- Arch Linux
- Parch Linux
- Manjaro
- Armbian
- AlmaLinux 9.5+
- Rocky Linux 9.5+
- Oracle Linux 8+
- OpenSUSE Tubleweed
- Amazon Linux 2023
- Windows x64 / x86 / ARM64
- macOS (Intel & Apple Silicon M1/M2/M3/M4)
- FreeBSD
- OpenBSD

## 支持的架构和设备

<details>
  <summary>点击查看 支持的架构和设备</summary>

我们的平台提供与各种架构和设备的兼容性，确保在各种计算环境中的灵活性。以下是我们支持的关键架构：

- **amd64**: 这种流行的架构是个人计算机和服务器的标准，可以无缝地适应大多数现代操作系统。

- **x86 / i386**: 这种架构在台式机和笔记本电脑中被广泛采用，得到了众多操作系统和应用程序的广泛支持，包括 Windows、macOS、Linux、FreeBSD 和 OpenBSD 系统。

- **armv8 / arm64 / aarch64**: 这种架构专为现代移动、桌面和嵌入式设备（Apple Silicon M1/M2/M3/M4、Raspberry Pi 4/5、Orange Pi 3/5 等）量身定制。

- **armv7 / arm / arm32**: 作为移动和嵌入式设备的架构，广泛用于 Orange Pi Zero LTS、Orange Pi PC Plus、Raspberry Pi 2/3 等设备。

- **armv6 / arm / arm32**: 这种架构面向低功耗嵌入式设备（Raspberry Pi 1、Raspberry Pi Zero/Zero W）。

- **armv5 / arm / arm32**: 主要与早期嵌入式系统相关的旧架构。

- **mips64 / mips64le / mips32 / mips32le**: 用于路由器、网络硬件（OpenWrt/嵌入式 Linux 平台）和企业系统的 MIPS 架构。

- **riscv64**: 用于现代嵌入式硬件和单板计算机（SiFive、Milk-V、StarFive）的 64 位 RISC-V 开源指令集架构。

- **ppc64 / ppc64le**: 用于企业级服务器（IBM Power 系统）的 64 位 PowerPC 架构。

- **s390x**: 常用于 IBM 大型机计算机，为企业工作负载提供高性能和可靠性。

- **loong64**: 用于龙芯平台的 64 位 LoongArch 架构。

</details>

## Languages

- Arabic (阿拉伯)
- English（英语）
- Persian（波斯语）
- Traditional Chinese（繁体中文）
- Simplified Chinese（简体中文）
- Japanese（日语）
- Russian（俄语）
- Vietnamese（越南语）
- Spanish（西班牙语）
- Indonesian（印尼语）
- Ukrainian（乌克兰语）
- Turkish（土耳其语）
- Português (Brazil)（葡萄牙语（巴西））

## Features

- **支持 AmneziaWG (AWG 3.1) 协议**：原生内核与运行时集成、可配置抗深度包检测 (DPI) 垃圾包混淆 (`jc`, `jmin`, `jmax`, `s1`-`s4`, `h1`-`h4`)、RFC 7748 Curve25519 密钥对生成、一键导出 `.conf` 客户端配置文件及 `vpn://` / `wireguard://` 分享链接与二维码。
- **集成 FakeTLS 的 MTProto 代理**：原生集成 `mtg` 多租户边车进程管理器、自动化 FakeTLS 域名伪装密钥生成 (`ee...`/`dd...`)、一键生成 Telegram 专属直连链接 (`tg://proxy`, `t.me/proxy`) 及二维码。
- **External Proxy 外部接入端点覆盖**：支持在 WireGuard/AmneziaWG `.conf` 配置文件 (`Endpoint = dest:port`)、`vpn://` / `wireguard://` 链接与 Telegram `tg://proxy` 链接中直接应用外部代理域名与端口。
- **内置交互式面板 API 文档 (`/panel/docs`)**：在网页端直接浏览、检索端点、一键复制路径并查看实时 cURL 与多语言调用示例。
- **配置状态自动与手动回滚**：前置配置测试验证、核心即时退出崩溃监控，发生异常时自动降级恢复至 `config.json.last_working` 确保服务不中断。
- **最新 Xray-core 核心 (v26.10.21)**：完整支持 Finalmask 源 IP ACL 过滤 (`allowedSourceIPs`/`blockedSourceIPs`)、客户端分享链接自动剔除服务器端过滤、XDRIVE 传输增强与扩展 TLS 密码套件。
- **协议密码与密钥完全独立生成与保存**：针对 Trojan、Shadowsocks 2022（严格自动修正 16/32 字节 Base64 密钥与防错）、MASQUE、Hysteria、MTProto 及 WireGuard/AmneziaWG 提供独立的凭据生成与持久化存储。
- **已连接设备跟踪与管理**：记录每个客户端的活跃连接设备（IP、客户端软件、操作系统及最后活跃时间），支持一键清理。
- **数据库健康诊断与维护**：面板内置 SQLite 状态体检、WAL 截断、PRAGMA 优化、空间释放 (VACUUM) 与孤立破损记录清理。
- **强制使用 HTTPS 安装与访问面板**
- **系统状态监控**（CPU、内存、Swap、磁盘、网络实时吞吐量与连接数）
- **在所有入站和客户端中搜索与筛选**
- **深色、极黑 (Ultra Dark) 与浅色主题**
- **支持多用户和多协议**：VMess、VLESS、Trojan、Shadowsocks、WireGuard、AmneziaWG (AWG 3.1)、MTProto (FakeTLS)、MASQUE、Hysteria、Dokodemo-door、Socks、HTTP
- 支持 XTLS 原生协议：RPRX-Direct、Vision、REALITY
- 流量统计、流量配额限制、过期时间限制
- 可自定义的 Xray 配置模板
- 支持一键式 SSL 证书申请与自动续期
- 完整 100% 国际化多语言支持（涵盖 13 种语言）
- 支持从面板导出与导入数据库
- 内置面板应用更新程序

## 默认面板设置

<details>
  <summary>点击查看默认设置详情</summary>

### 用户名、密码、端口和 Web Base Path

如果您选择不修改这些设置，它们将随机生成（不适用于 Docker）。

**Docker 的默认设置：**

- **用户名：** admin
- **密码：** admin
- **端口：** 2053

### 数据库管理：

您可以直接在面板中方便地进行数据库备份和还原。

- **数据库路径:**
    - `/etc/x-ui/x-ui.db`

### Web 基础路径

1. **重置 Web 基础路径:**
    - 打开终端。
    - 运行 `x-ui` 命令。
    - 选择 `重置 Web 基础路径` 选项。

2. **生成或自定义路径:**
    - 路径将会随机生成，或者您可以输入自定义路径。

3. **查看当前设置:**
    - 要查看当前设置，请在终端中使用 `x-ui settings` 命令，或在 `x-ui` 面板中点击 `查看当前设置`。

### 安全建议：

- 为了提高安全性，建议在URL结构中使用一个长的随机词。

**示例：**

- `http://ip:port/*webbasepath*/panel`
- `http://domain:port/*webbasepath*/panel`

</details>

## WARP 配置

<details>
  <summary>点击查看 WARP 配置详情</summary>

#### 使用方法

**对于 `v2.1.0` 及之后的版本：**

WARP 已内置，无需额外安装。只需在面板中开启相关配置即可。

</details>

## IP 限制

<details>
  <summary>点击查看 IP 限制详情</summary>

#### 使用方法

**注意:** 当使用 IP 隧道时，IP 限制将无法正常工作。

- **对于 `v1.6.1` 及之前的版本：**
    - IP 限制功能已内置于面板中。

**对于 `v1.7.0` 及更新的版本：**

要启用 IP 限制功能，您需要安装 `fail2ban` 及其所需的文件，步骤如下：

1. 在终端中运行 `x-ui` 命令，然后选择 `IP 限制管理`。
2. 您将看到以下选项：

    - **更改封禁时长:** 调整封禁时长。
    - **解除所有封禁:** 解除当前的所有封禁。
    - **查看日志:** 查看日志。
    - **Fail2ban 状态:** 检查 `fail2ban` 的状态。
    - **重启 Fail2ban:** 重启 `fail2ban` 服务。
    - **卸载 Fail2ban:** 卸载带有配置的 Fail2ban。

3. 在面板中通过设置 `Xray 配置/log/访问日志` 为 `./access.log` 添加访问日志路径，然后保存并重启 Xray。

- **对于 `v2.1.3` 之前的版本：**
    - 您需要在 Xray 配置中手动设置访问日志路径：

      ```sh
      "log": {
        "access": "./access.log",
        "dnsLog": false,
        "loglevel": "warning"
      },
      ```

- **对于 `v2.1.3` 及之后的版本：**
    - 面板中直接提供了配置 `access.log` 的选项。

</details>

## Telegram 机器人

<details>
  <summary>点击查看 Telegram 机器人</summary>

#### 使用

Web 面板通过 Telegram Bot 支持每日流量、面板登录、数据库备份、系统状态、客户端信息等通知和功能。要使用机器人，您需要在面板中设置机器人相关参数，包括：

- 电报令牌
- 管理员聊天 ID
- 通知时间（cron 语法）
- 到期日期通知
- 流量上限通知
- 数据库备份
- CPU 负载通知

**参考：**

- `30 * * * * *` - 在每个点的 30 秒处通知
- `0 */10 * * * *` - 每 10 分钟的第一秒通知
- `@hourly` - 每小时通知
- `@daily` - 每天通知 (00:00)
- `@weekly` - 每周通知
- `@every 8h` - 每8小时通知

### Telegram Bot 功能

- 定期报告
- 登录通知
- CPU 阈值通知
- 提前报告的过期时间和流量阈值
- 如果将客户的电报用户名添加到用户的配置中，则支持客户端报告菜单
- 支持使用UUID（VMESS/VLESS）或密码（TROJAN）搜索报文流量报告 - 匿名
- 基于菜单的机器人
- 通过电子邮件搜索客户端（仅限管理员）
- 检查所有入库
- 检查服务器状态
- 检查耗尽的用户
- 根据请求和定期报告接收备份
- 多语言机器人

### 注册 Telegram bot

- 与 [Botfather](https://t.me/BotFather) 对话：
  ![Botfather](./media/botfather.png)

- 使用 /newbot 创建新机器人：你需要提供机器人名称以及用户名，注意名称中末尾要包含“bot”
  ![创建机器人](./media/newbot.png)

- 启动您刚刚创建的机器人。可以在此处找到机器人的链接。
  ![令牌](./media/token.png)

- 输入您的面板并配置 Telegram 机器人设置，如下所示：
  ![面板设置](./media/panel-bot-config.png)

在输入字段编号 3 中输入机器人令牌。
在输入字段编号 4 中输入用户 ID。具有此 id 的 Telegram 帐户将是机器人管理员。 （您可以输入多个，只需将它们用“ ，”分开即可）

- 如何获取TG ID? 使用 [bot](https://t.me/useridinfobot)， 启动机器人，它会给你 Telegram 用户 ID。
  ![用户 ID](./media/user-id.png)

</details>

## API 路由

<details>
  <summary>点击查看 API 路由</summary>

#### 完整文档

- **内置交互式面板文档**：可直接在 TX-UI 面板的 `/panel/docs` 页面查看并交互测试。
- **完整 API 规范与多语言代码示例**：参阅 [API.md](API.md)
- [Postman 接口集合](https://www.postman.com/incognito-coder/tx-ui/collection/q1l5l0u/tx-ui)
- 身份认证：向 `/login` 发送 `POST` 请求（携带 `{ "username": "admin", "password": "..." }`）即可自动设置会话 Cookie。

#### 客户端接口 (`/panel/api/clients`)

| 方法 | 路径 | 说明 |
|:---:|------|-----|
| `GET` | `"/list"` | 获取所有客户端及其流量统计、过期时间与入站绑定信息 |
| `GET` | `"/get/:id"` | 根据唯一 ID 获取指定客户端配置 |
| `POST` | `"/create"` | 新建客户端并指定专属协议凭据 (`password`, `ssPassword`, `masquePass`, `auth`, `secret`, `privateKey`, `publicKey`, `preSharedKey`) |
| `POST` | `"/bulkCreate"` | 批量生成客户端 |
| `POST` | `"/update/:id"` | 更新客户端配置与配额 |
| `POST` | `"/del/:id"` | 删除客户端 |
| `POST` | `"/bulkDel"` | 批量删除客户端 |
| `POST` | `"/:id/toggle"` | 快捷启用或禁用客户端 |
| `GET` | `"/:id/links"` | 获取客户端关联绑定的所有入站 ID |
| `POST` | `"/:id/addLink"` | 将客户端关联至指定入站 |
| `POST` | `"/:id/setLinks"` | 批量重置并替换客户端入站绑定 |
| `POST` | `"/:id/removeLink/:inboundId"` | 解除客户端与指定入站的绑定 |
| `GET` | `"/:id/traffic"` | 获取客户端实时上行与下行流量统计 |
| `POST` | `"/:id/resetTraffic"` | 重置客户端已用流量并在未过期时恢复启用 |
| `POST` | `"/resetAllTraffics"` | 重置所有客户端的流量计数器 |
| `POST` | `"/delDepleted"` | 清理删除所有流量耗尽或已过期的客户端 |
| `GET` | `"/:id/devices"` | 获取客户端的已连接设备历史（IP、系统、应用等） |
| `POST` | `"/:id/clearDevices"` | 清空客户端已连接设备记录 |

#### 入站接口 (`/panel/api/inbounds`)

| 方法 | 路径 | 说明 |
|:---:|------|-----|
| `GET` | `"/list"` | 获取所有代理入站规则 |
| `GET` | `"/get/:id"` | 根据入站 ID 获取详情 |
| `GET` | `"/getClientTraffics/:email"` | 根据邮箱获取客户端流量 |
| `GET` | `"/getClientTrafficsById/:id"` | 根据 ID 获取客户端流量 |
| `POST` | `"/add"` | 新增代理入站配置 |
| `POST` | `"/del/:id"` | 删除指定入站规则 |
| `POST` | `"/update/:id"` | 更新指定入站规则 |
| `POST` | `"/reorder"` | 调整入站规则排序 |
| `POST` | `"/clientIps/:email"` | 获取客户端连接 IP 记录 |
| `POST` | `"/clearClientIps/:email"` | 清除客户端连接 IP 记录 |
| `POST` | `"/updateClientTraffic/:email"` | 调整客户端流量配额（单位为字节） |
| `POST` | `"/:id/resetClientTraffic/:email"` | 重置入站中指定客户端的流量 |
| `POST` | `"/resetAllTraffics"` | 重置所有入站的汇总流量 |
| `POST` | `"/resetAllClientTraffics/:id"` | 重置入站中所有客户端的流量 (-1 表示全部) |
| `POST` | `"/delDepletedClients/:id"` | 删除入站中耗尽/过期的客户端 (-1 表示全部) |
| `POST` | `"/onlines"` | 获取当前在线的客户端邮箱列表 |
| `POST` | `"/depleted"` | 获取已耗尽配额的客户端邮箱列表 |
| `POST` | `"/disabled"` | 获取已禁用的客户端邮箱列表 |

#### 系统与服务接口 (`/panel/api/server`)

| 方法 | 路径 | 说明 |
|:---:|------|-----|
| `GET` | `"/status"` | 获取 CPU、内存、磁盘状态及 Xray 运行状况 |
| `POST` | `"/restartXrayService"` | 重启 Xray 核心服务 |
| `POST` | `"/stopXrayService"` | 停止 Xray 核心服务 |
| `POST` | `"/rollbackXrayConfig"` | 发生错误时回滚 Xray 配置至最后正常工作状态 |
| `GET` | `"/getRollbackStatus"` | 获取当前回滚状态及触发原因 |
| `GET` | `"/getXrayVersion"` | 获取当前已安装的 Xray 核心版本 |
| `GET` | `"/getConfigJson"` | 获取正在运行的激活 config.json 文件 |
| `GET` | `"/getDb"` | 下载 SQLite 数据库备份文件 |
| `POST` | `"/importDB"` | 通过上传 .db 文件恢复数据库 |
| `POST` | `"/logs/:count"` | 获取面板最新日志记录 |
| `POST` | `"/xraylogs/:count"` | 获取 Xray 核心最新日志记录 |
| `GET` | `"/getNewUUID"` | 生成全新 UUID v4 |
| `GET` | `"/getNewX25519Cert"` | 生成 Reality X25519 密钥对 |
| `POST` | `"/installPanel"` | 更新或安装面板程序包 |

#### 设置与数据库维护接口 (`/panel/setting`)

| 方法 | 路径 | 说明 |
|:---:|------|-----|
| `POST` | `"/all"` | 获取全部面板配置项 |
| `POST` | `"/update"` | 保存更新面板配置项 |
| `POST` | `"/restartPanel"` | 平滑重启面板 Web 进程 |
| `POST` | `"/dbStatus"` | 检查 SQLite 数据库大小、WAL 日志与完整性 |
| `POST` | `"/optimizeDatabase"` | 执行数据库优化 (WAL 截断与 PRAGMA 优化) |
| `POST` | `"/vacuumDatabase"` | 碎片整理并通过 VACUUM 释放磁盘空间 |
| `POST` | `"/cleanDatabase"` | 扫描并清理孤立破损数据行 |

#### Xray 高级配置接口 (`/panel/xray`)

| 方法 | 路径 | 说明 |
|:---:|------|-----|
| `POST` | `"/"` | 获取完整的高级 Xray 模板配置 |
| `POST` | `"/update"` | 保存并应用高级 Xray 规则 |
| `POST` | `"/rollback"` | 在 Xray 设置中回滚至上一次正常工作配置 |
| `GET` | `"/getRollbackStatus"` | 查询回滚标识与详细原因 |
| `POST` | `"/testOutbound"` | 测试出站连通性与网络延迟 |
| `GET` | `"/getOutboundsTraffic"` | 获取出站标签累计流量 |
| `POST` | `"/resetOutboundsTraffic"` | 重置出站流量计数器 |

- [<img src="https://run.pstmn.io/button.svg" alt="Run In Postman" style="width: 128px; height: 32px;">](https://app.getpostman.com/run-collection/5146551-dda3cab3-0e33-485f-96f9-d4262f437ac5?action=collection%2Ffork&source=rip_markdown&collection-url=entityId%3D5146551-dda3cab3-0e33-485f-96f9-d4262f437ac5%26entityType%3Dcollection%26workspaceId%3Dd64f609f-485a-4951-9b8f-876b3f917124)

</details>

## 环境变量

<details>
  <summary>点击查看 环境变量</summary>

#### Usage

| 变量             |                  Type                  | 默认          |
|----------------|:--------------------------------------:|:------------|
| XUI_LOG_LEVEL  | "debug" \| "info" \| "warn" \| "error" | "info"      |
| XUI_DEBUG      |               `boolean`                | `false`     |
| XUI_BIN_FOLDER |                `string`                | "bin"       |
| XUI_DB_FOLDER  |                `string`                | "/etc/x-ui" |
| XUI_LOG_FOLDER |                `string`                | "/var/log"  |

例子：

```sh
XUI_BIN_FOLDER="bin" XUI_DB_FOLDER="/etc/x-ui" go build main.go
```

</details>

## 订阅 UI

您可以使用此代码库为您的面板创建订阅 UI [TX-UI 主题中心](https://github.com/Incognito-Coder/TX-ThemeHub)

## 致谢

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**): _Enhanced v2ray/xray and
  v2ray/xray-clients routing rules with built-in Iranian domains and a focus on security and adblocking._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**): _This repository
  contains automatically updated V2Ray routing rules based on data on blocked domains and addresses in Russia._

## Star趋势

[![Stargazers over time](https://starchart.cc/Incognito-Coder/tx-ui.svg?variant=adaptive)](https://starchart.cc/Incognito-Coder/tx-ui)
