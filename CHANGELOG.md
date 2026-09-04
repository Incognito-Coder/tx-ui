# Changelog

All notable changes to **TX-UI** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
