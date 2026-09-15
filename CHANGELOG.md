# Changelog

All notable changes to **TX-UI** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v0.8.4] - 2026-09-15

### 🚀 Features & Enhancements
- **Progressive Web App (PWA) Support**:
  - Full PWA integration allowing native standalone app installation across mobile (Android, iOS) and desktop (Chrome, Edge, macOS, Windows).
  - Web App Manifest (`manifest.json`) supporting standalone display mode, dynamic theme color (`#2563eb`), dark background (`#0a1222`), and orientation flexibility.
  - Smart Service Worker (`sw.js`) with cache management: strictly network-only for authentication and API endpoints (`/api/*`, `/login`, `/logout`), stale-while-revalidate for static assets, and network-first navigation with custom dark-themed offline fallback (`offline.html`).
  - Generated comprehensive PWA app icon suite derived from `media/tx-ui-dark.png`: standard 192x192 & 512x512 icons, Android maskable icons with safe zone margins, Apple touch icon (180x180), and multi-resolution favicon.
- **Built-in Panel Updater & Version Selector**:
  - Enhanced `UpdatePanel` API and settings modal to fetch, compare, and display all available newer panel releases directly from GitHub.
  - Interactive release version selector modal allowing one-click upgrades to specific panel releases.
  - Optimized release checking routine to prevent background CPU leakage on panel start or restart.
- **Inbound Clients Sorting**:
  - Added ascending and descending sort options for inbounds client list, enabling flexible ordering by traffic, expiry, or email.

### 🐛 Bug Fixes & Stability
- **Node Client Traffic Accounting & Volume Unification**:
  - Unified traffic quota calculation across all linked inbounds: total volume now strictly adheres to the configured client limit instead of summing up separate quotas per inbound.
  - Synchronized upstream and downstream usage across all linked inbounds for Node Clients, ensuring accurate quota enforcement and simultaneous deactivation when the total volume limit is exhausted.
- **Node Client QR Links Modal UI**:
  - Fixed modal card margins, centered layout alignment, and resolved previous right-side alignment caused by `.qr-modal { align-items: flex-end }`.
  - Added responsive modal sizing (`panel-modal-auto panel-modal-qr`) preventing unwanted 100vw stretch on mobile.
  - Styled full-width matching Copy buttons with rounded corners and centered icons.
- **Geo Assets Version Persistence**:
  - Resolved issue where geodata version displayed as "Unknown" following panel updates or reinstalls.
  - Added automatic detection and version preservation across reinstallations.
- **Docker Infrastructure & Buildx**:
  - Fixed Docker container build error by updating base image to `debian:bookworm-slim`.
  - Improved entrypoint script POSIX compatibility for containerized deployments.
- **Xray Telemetry & Metrics UI**:
  - Fixed missing telemetry icon assets and improved traffic metric counters parsing and formatting.

### 🌐 Internationalization (i18n)
- Added `pages.index.latestVersion` translation key across all supported locales (`ar_EG`, `en_US`, `es_ES`, `fa_IR`, `id_ID`, `ja_JP`, `pt_BR`, `ru_RU`, `tr_TR`, `uk_UA`, `vi_VN`, `zh_CN`, `zh_TW`).

### 📦 Dependencies & Maintenance
- Updated project Go dependencies to latest versions in `go.mod` and `go.sum`.
- Updated release action versions and project documentation wallet addresses.

---

## [v0.8.3] - 2026-09-14

### 🚀 Features & Enhancements
- **WireGuard Inbounds**:
  - Standard `wireguard://` URI link generation for subscription feeds, ensuring remark preservation and seamless import in modern clients (Sing-box, v2rayNG, MahsaNG, NekoBox).
  - Added one-click WireGuard Share URL in the inbound details modal.
- **Geo Assets Release Picker**:
  - Interactive version selector modal previewing and selecting from the latest 10 releases of `Loyalsoldier/v2ray-rules-dat` (`geoip.dat` & `geosite.dat`).
- **Xray Metrics & Telemetry**:
  - Added **Clean Inspector** tab with categorized sections (Runtime, Memory, Traffic Counters).
  - Added **Raw Inspector & Watch** mode with live polling and formatted JSON inspection.
  - Modernized search inputs to match panel dark theme aesthetic.
- **UI & UX Polish**:
  - Added numeric inbound ID column to mobile table view and info popover.
  - Enforced non-wrapping horizontal-scroll view (`white-space: pre`) in System Logs and Xray Logs modals.

### 🌐 Internationalization (i18n)
- Added native translations across all 13 supported locales (`ar_EG`, `en_US`, `es_ES`, `fa_IR`, `id_ID`, `ja_JP`, `pt_BR`, `ru_RU`, `tr_TR`, `uk_UA`, `vi_VN`, `zh_CN`, `zh_TW`) for Running state, Xray Logs, Geo asset release selection, DNS, Tunnel, and Fake DNS.
- Added automated `TestI18nTranslations` unit test ensuring complete translation coverage.

### 🐛 Bug Fixes
- **Duplicate Protocol**: Fixed duplicate `wireguard` entry in Add/Edit Inbound protocol dropdown.
- **Metrics Crash**: Resolved client-side crash when Xray traffic stats are empty or uninitialized.
- **Goroutines Zero Value**: Fixed goroutines count always reporting zero by parsing `pprof` goroutine header.

---

## [v0.8.2] - 2026-09-10

### 🚀 Features & Xray-core Integration
- **Xray-core v26.9.9 Upgrade**: Updated `xray-core` dependency to latest `v26.9.9` release tag across `go.mod`, `DockerInit.sh`, and GitHub Actions release workflow (`release.yml`).
- **Xray-core v26.3.27 - v26.9.9 Feature Alignment**:
  - **Finalmask Obfuscation Engine**: Full panel support for Finalmask TCP & UDP masks including `udpHop` (`ports`, `interval`), `Sudoku` (`ascii`, `paddingMin/Max`), `fragment` (`length`, `interval`), `noise`, `header-custom`, and `salamander` in both inbound and outbound form models.
  - **mKCP TTI Extension**: Expanded mKCP Transmission Time Interval (TTI) max range from 100 ms up to 5000 ms for XDNS and high-latency connections.
  - **REALITY Security Hints**: Added real-time security warning alert on REALITY configuration forms for non-443 target ports or Apple/iCloud SNI domains.
- **VLESS Post-Quantum Encryption (VLESS ENC)**: Refined `GetNewVlessEnc()` in `server.go` to support `ML-KEM-768` (Post-Quantum) and `X25519` keypair generation, added selection UI in `vless.html`, and updated subscription URL generation in `inbound.js`.
- **Routing Rules Enhancement**: Added `localIP` and `localPort` inputs to `xray_rule_modal.html` alongside `vlessRoute` and `sourceIP`.
- **Docker Infrastructure Modernization**: Upgraded Docker base image to Debian Bookworm (`debian:bookworm-slim`), migrating package management from `apk` to `apt-get` for improved stability and glibc binary compatibility.

### 🌐 Internationalization (i18n)
- **100% Translation Coverage**: Audited 572 template i18n keys and added missing translation strings across all 13 supported locales (`ar_EG`, `en_US`, `es_ES`, `fa_IR`, `id_ID`, `ja_JP`, `pt_BR`, `ru_RU`, `tr_TR`, `uk_UA`, `vi_VN`, `zh_CN`, `zh_TW`).

### 🐛 Bug Fixes & Improvements
- **VLESS Link Generation Fix**: Resolved `ReferenceError: settings is not defined` bug in `genVLESSLink()` in `inbound.js`.
- **Xray Release Fetching**: Fixed `GetXrayVersions()` in `server.go` by sending `User-Agent: tx-ui/1.0` header and using full stream reader to prevent truncated JSON response syntax errors from GitHub API.

---

## [v0.8.1] - 2026-09-04

### 🚀 Features
- **Internationalization (i18n)**: Added full i18n translation support across 15 languages for GitHub Sub HTML Template Manager (`1abed0bf`) and Live Xray Metrics Telemetry Dashboard (`0901227f`).

### 🐛 Bug Fixes
- **Goroutine & Telemetry Metrics Parsing**: Fixed zero goroutines thread count and memory stats parsing in `extractMetricsSummary()`; added HTTP status check and fallback from `/debug/vars` to `/metrics`. (`d4814fcc`)
- **V-002 Security Vulnerability**: Resolved V-002 security vulnerability issue. (`8e5fcb87`)
- **Windows CGO Build Pipeline**: Set `CGO_ENABLED=0` for Windows release binaries in GitHub CI workflow to resolve `windows-386` compilation failures. (`1832b4b2`)

### 🎨 Styling & Theme Enhancements
- **Dynamic Theme Accent Synchronization**: Synchronized system progress bars, usage meters, and back-to-top floating buttons with the active panel theme color. (`3afdaaa0`)

### 📦 Dependencies & Maintenance
- **Dependency Upgrades**: Upgraded Go module dependencies in `go.mod` (`gopsutil/v4`, `fasthttp`, `gorm`, `grpc`, `golang.org/x/crypto`, etc.) while preserving pinned `gvisor` version. (`41a4a260`)
- **Workspace Cleaning**: Ignored local `sub/` directory in `.gitignore` and removed obsolete asset references. (`58249aa2`, `f1424aff`)

---

## [v0.8.0] - 2026-09-04

### 🚀 Features
- **Multi-User WireGuard Support**: Added complete multi-user WireGuard protocol integration with client management, auto-fill address capabilities, subscription links, and individual client traffic stats. (`a7d72c2f`)
- **Subscription HTML Template Manager**: Integrated GitHub Sub HTML template manager with live sync from `TX-ThemeHub`, automatic service restart, and live theme switcher. (`5a462617`)
- **Xray Metrics & Telemetry Dashboard**: Added Xray metrics background collection service, live telemetry modal dashboard, and panel CSS responsive styling. (`d6552eba`)
- **Dynamic Panel Theme Color**: Introduced panel theme color selection with custom tag, tab, and ink-bar theme styling across 15 translations. (`cc4e1540`)
- **Self-Signed Cert Generator & Telegram Topics**: Added built-in self-signed SSL/TLS certificate generator, Telegram topic ID support, and notify-only mode. (`f79cf3e0`)
- **Expanded OS & Architecture Support**: Added support for FreeBSD and OpenBSD operating systems, plus MIPS, RISC-V, PPC, and LoongArch CPU architectures. (`5b1c3e56`)

### 🐛 Bug Fixes
- **Windows Compatibility**: Replaced POSIX `SIGHUP` signal handling with portable Go channel restart logic to enable seamless panel restarts on Windows OS. (`fe513e8b`)
- **WireGuard Interface Normalization**: Normalized WireGuard server interface address as a slice to prevent Xray-core crashes when adding clients. (`f268d868`)
- **Database & Client Sync**: Set `clients` key as single source of truth in DB settings with peer fallback mechanisms during client synchronization. (`d3e68381`)
- **Xray Engine & Parsing**: Added missing ciphers, network sniffing options, and safe stream parsing for Xray-core compatibility. (`97a53a1f`)
- **Sidebar & Navigation Fixes**: Eliminated layout shifts and flickering on page navigation; preserved sider collapsed state across sessions. (`62a40a3d`, `20246bdf`)
- **Login Theme & Mobile Graphics**: Persisted login theme color across panel navigation and fixed mobile login wave graphics. (`5fce706c`)
- **UI Glitches & QR Peer Fallback**: Fixed protocol case alias handling, QR code peer fallback, ant-collapse border-radius, and modal tab ink-bar styling. (`52685ebb`, `f35ee575`)

### 🎨 Styling & UX
- **Page Entrance Animations**: Upgraded page load entrance animation with smooth fade-up & scale using `cubic-bezier` easing curves. (`68be2042`)

### 👷 CI & Build Pipelines
- **Cross-Platform Release Workflow**: Updated `.github/workflows/release.yml` to build binaries for Windows (`amd64`, `386`, `arm64`) and macOS (`amd64`, `arm64`) alongside Linux releases. (`ab3c32a6`)

### 📚 Documentation
- **Multi-Language README Updates**: Updated documentation across all language files (`README.md`, `README.fa_IR.md`, `README.zh_CN.md`, `README.ru_RU.md`, `README.es_ES.md`, `README.ar_EG.md`) reflecting expanded platform and architecture support. (`d17ad6f2`)

---
