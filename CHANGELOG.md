# Changelog

All notable changes to **TX-UI** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v0.8.8] - 2026-10-08

### 🚀 Features & Enhancements
- **Bulk Client Inbound Link Management**:
  - Added multi-select batch assignment (`bulkSetLinks`) allowing administrators to select multiple clients and associate or replace inbound links simultaneously (`04d72ea2`).
  - Added batch link management drawer and bulk action menu integration in the Clients view (`/panel/clients`).
- **Encrypted Client Hello (ECH) Generation & Propagation**:
  - Implemented automatic ECH certificate and keypair generation for TLS/Reality inbounds (`11b5d963`).
  - Automatically propagated generated ECH configurations into outbound settings and subscription endpoints.
- **REST API Reference Updates (`API.md`)**:
  - Corrected `POST /panel/api/clients/bulkCreate` payload specifications to accurately document `{ clients: [...], inboundIds: [...] }`.
  - Added documentation for `POST /panel/api/clients/bulkSetLinks`, client traffic reset endpoints, and depleted client deletion endpoints.

### 🐛 Bug Fixes & Stability
- **Inbound Deletion Client & Link Preservation**:
  - Preserved client traffic statistics and existing link bindings when deleting an individual inbound (`10a27e13`). Clients linked to multiple inbounds retain active records and traffic counters without unintended cascading deletions.
- **SQLite Concurrency & Traffic Reset Locks**:
  - Fixed intermittent `database is locked` errors during client traffic reset operations by synchronizing write queries with `nodeClientOpMutex` and atomic database transactions.
  - Separated external gRPC / Xray API calls from the SQLite transaction window to minimize database write lock hold times.
  - Fixed client modal reset button endpoint resolution to prevent malformed API requests when `inboundId` is omitted.
- **Depleted Client Deletion via API**:
  - Resolved issue where API requests (`/panel/api/inbounds/delDepletedClients/:id` and `/panel/api/clients/delDepleted`) failed to delete depleted node clients.
  - Removed restrictive `node_client_id IS NULL` filtering in `DelDepletedClients`, cascading removal to associated inbound settings JSON, node client links, and orphaned node client records.
  - Removed nested `s.DelInbound` calls inside open transactions to prevent database deadlocks.
- **"Obtain Failed: record not found" Toast Elimination**:
  - Handled `gorm.ErrRecordNotFound` gracefully in inbound controller methods (`getInbound` and `getInbounds`), returning clean empty responses and eliminating toast error popups on stale or missing records.
  - Preloaded `ClientStats` in `GetInbound` to ensure full client traffic statistics are immediately available on single inbound queries.
- **Total GB Input & Formatting**:
  - Fixed `[[ client._totalGB + "GB" ]]` displaying `"undefinedGB"` on plain client objects in inbound client tables by formatting with `sizeFormat(client.totalGB)`.
  - Fixed Vue 2 reactivity when opening client edit modals by initializing `_totalGB` inside `Object.assign`.
  - Added `:precision="2"` and `v-model.number` to total GB inputs across client modals, preventing backspace from snapping directly to `0`.
  - Safeguarded `Inbound.ClientBase._totalGB` getter and setter against `NaN` and `null` values with integer byte rounding on save.
- **Clean Theme Session Synchronization**:
  - Fixed hybrid light/dark theme collision occurring on new browser sessions where `localStorage` was uninitialized.
  - Standardized fallback theme to dark mode across both `head.html` and `themeSwitch.html`.
  - Synchronized `.dark` and `.light` classes simultaneously across `document.documentElement`, `document.body`, and `#app` both on initialization and during theme toggle.

### ⚡ Performance & Database Optimization
- **Batch Traffic Aggregation for Depleted Client Queries**:
  - Optimized `NodeClientService.DeleteDepleted` by replacing the O(N) query loop with a single grouped aggregate SQL query, eliminating N+1 database queries.
- **Optimized Link Counter Scans**:
  - Removed unnecessary join on `inbounds` in `GetLinkedInboundsCounts`, directly querying `node_client_links` with indexed group scanning.
- **Bulk Operation Mutex Protection**:
  - Guarded bulk client linking and deletion operations with `nodeClientOpMutex` to prevent concurrent transaction collision (`9c02e220`).

---

## [v0.8.7] - 2026-10-03

### 🚀 Features & Enhancements
- **Client Traffic Reset in Edit Modals**:
  - Added a direct "Reset Traffic" action button and real-time upload/download usage indicators inside the Client Edit Modal (both Node Clients `/panel/clients` and Inbound Clients modal `/panel/inbounds`).
  - Resets usage (`up = 0`, `down = 0`) reactively in both the database and UI without requiring a full page refresh.
  - Automatically re-enables the client across both `NodeClient` and `ClientTraffic` records when reset (provided the client is not expired), preventing background jobs from immediately re-exhausting the client.
  - Robustly matches traffic records by both `node_client_id` and email, ensuring legacy records with NULL IDs are properly reset.
- **Subscription JSON URL in Client Info**:
  - Added Sub JSON URL row (`subJsonURI + subId`) with a one-click copy button to the Client Details modal (`detailModal`) alongside standard subscription links.
- **Outbound Modal Protocol Dropdown Duplicate Fix**:
  - Defined uppercase alias keys (`VMESS`, `TROJAN`, `SHADOWSOCKS`, `WIREGUARD`, `HYSTERIA`, `MASQUE`) as non-enumerable properties on `Protocols` in `outbound.js`, preventing duplicate uppercase items (e.g. `Trojan` and `TROJAN`) from displaying in the Outbound modal protocol dropdown selector.

### 🐛 Bug Fixes & Stability
- **Immediate Core Restart on Traffic Limit Exhaustion**:
  - Restored immediate Xray core process restart (`RestartXray(true)`) upon client quota or expiration limit detection in traffic collection jobs (`XrayTrafficJob`), ensuring active TCP/WebSocket/gRPC connections for exhausted clients are terminated immediately.
- **Inbound Deletion Link Cleanup & Stale Link Removal**:
  - Fixed orphaned links remaining when deleting an inbound: `DelInbound` now automatically cleans up all associated `node_client_links` and removes orphaned `node_clients` with zero remaining links.
  - Joined `inbounds` table in `GetAllWithDetails`, `GetLinks`, and `GetLinkedInboundsCounts` to filter out links pointing to non-existent inbounds, and updated client modal link selection logic.
- **Client Count Deduplication**:
  - Deduplicated total client counts and status popovers in `inbounds.html` (`total.clients`, `total.deactive`, `total.depleted`, `total.expiring`) to prevent clients linked to multiple inbounds from being counted multiple times in top summary metrics.
- **Log Viewer Debug Level Formatting**:
  - Fixed log level trimming and case conversion in `logger.GetLogs` and corrected `levelIndex` lookup in `logModal.formatLogs` (`index.html`) so `DEBUG` logs display with proper formatting in the panel log viewer.
- **CommonClass toHeaders TypeError Fix**:
  - Added missing `toHeaders` and `toV2Headers` static helper methods to `CommonClass` in `outbound.js`, resolving runtime `TypeError: CommonClass.toHeaders is not a function` when parsing stream settings (such as Masque headers).
- **Total Flow GB Stepper Fix**:
  - Fixed `<a-input-number>` stepper for the Total Flow (GB) field in client modals (`form/client.html`, `clients.html`). The up/down arrows and typed input now correctly update the underlying `totalGB` value.
  - Root cause: Vue 2 does not observe prototype getter/setter pairs (`_totalGB`) reactively through `v-model`; replaced with explicit `:value` + `@change` binding and added `:precision="2"` for proper decimal GB support.
  - Fixed client renewal via API keeping old expiration timestamps: `NodeClientController.update` now merges partial updates and handles snake_case JSON keys (`expiry_time`, `total_gb`).
  - Bidirectionally synchronized `expiry_time`, `total_gb`, `enable`, and `reset` between `node_clients`, `client_traffics`, and inbound settings JSON during client updates and renewals.
  - Synchronized group traffic calculations and auto-renew jobs (`autoRenewClients` and `NodeClientService.AutoRenew`) to update `node_clients` alongside `client_traffics`.
- **Dynamic Client Provisioning & Inbound Sync**:
  - Dynamically add and remove clients on active inbounds via Xray API without service interruption.
  - Synchronized `inbounds.settings` JSON with `node_client_links` to prevent resurrection or migration inconsistencies upon restart.
- **Modal Stepper Input Fix**:
  - Fixed numeric stepper `<a-input-number>` controls across all client and inbound modals (`clients.html`, `client_modal.html`, `client_bulk_modal.html`, `form/client.html`, `form/inbound.html`) where clicking the up/down arrows failed to increment or decrement Total GB flow.

### ⚡ Performance & Database Optimization
- **SQLite Concurrency & Lock Prevention**:
  - Enabled SQLite Write-Ahead Logging (`PRAGMA journal_mode = WAL;`), increased `busy_timeout` to 10 seconds (`10000ms`), and set `PRAGMA synchronous = NORMAL;` to eliminate `database is locked` / `database is busy` errors under concurrent operations.
  - Configured Go database connection pool (`SetMaxOpenConns(25)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(time.Hour)`) to avoid transaction deadlocks while maximizing read throughput.
- **Logging & Core Performance**:
  - Optimized internal logger with early-return short-circuits on non-debug levels, thread-safe buffering, and precompiled regexes in log writers to minimize CPU and allocation overhead.

### 🎨 UI & Mobile Polish
- **Mobile Long Link Text Wrapping**:
  - Added robust word wrapping (`word-break: break-all; overflow-wrap: anywhere; white-space: normal`) for long subscription, Sub JSON, and configuration URLs in Client Info (`clients.html`) and Inbound Info (`inbound_info_modal.html`) modals to prevent text overflow on mobile screens.
- **Inbounds Page Simplification**:
  - Cleaned up Inbounds management page (`inbounds.html`) by removing the redundant filter switch and radio controls, maintaining a cleaner search bar interface.

---

## [v0.8.6] - 2026-10-01

### 🚀 Features & Enhancements
- **Client Auto-Refresh Controls**:
  - Added configurable auto-refresh interval selector (5s to 60s) and toggle switch to the Clients management page (`/panel/clients`), matching Inbounds refresh behavior and persisting user preferences in `localStorage`.
  - Guarded client polling with `isLoadingClients` to prevent concurrent overlapping fetches during active refreshes.
- **Mobile Responsive Optimizations**:
  - Hid "Disabled" and "Ended" (`depleted`) status tag badges on client name and near the traffic progress bar on mobile viewports (`max-width: 768px`) to keep the compact mobile table clean and prevent row stretching.
  - Added spacing and margins between auto-refresh switches and controls across both clients and inbounds pages.
- **REST API Documentation (`API.md`)**:
  - Added full REST API reference documentation covering authentication, system endpoints, server settings, inbounds, clients, subscriptions, and database backups.

### 🐛 Bug Fixes & Stability
- **Inbound Client & Database Synchronization**:
  - Fixed issue where deleted clients were resurrected on panel restart: `nodeClientService.deleteInTx()`, `RemoveLink()`, and `SetLinks()` now strip deleted/unlinked client credentials from `inbounds.settings` JSON (`clients` and `peers`), preventing `MigrateLegacyClients()` from re-importing them on startup.
  - Purged associated `client_traffics` records upon client deletion to prevent orphaned statistics.
  - Fixed client deletion abort in `DelInboundClient` when no traffic record exists by ignoring `gorm.ErrRecordNotFound`.
  - Added automatic unlinking and linking of `NodeClient` records in `UpdateInbound` when modifying clients through the inbound modal.
  - Fixed client visibility when adding clients via the legacy `/addClient` API: `AddInboundClient` now automatically links and synchronizes clients with `node_clients` and `node_client_links`.
  - Fixed client deletion via API: `DelInboundClient` and `DelInboundClientByEmail` now fully delete or unlink `NodeClient` records to prevent orphaned entries from remaining in the panel.
- **PWA Offline Asset & Logo Caching**:
  - Replaced corrupted base64 fallback icon in `offline.html` with verified, crisp 192x192 TX-UI application icon.
  - Updated Service Worker (`sw.js`) to cache version `v3` and added all application logo icons (`tx-ui-dark.png`, standard/maskable 192x192 and 512x512 icons, favicons, `manifest.json`) to `PRECACHE_ASSETS`.
  - Expanded asset interceptor to handle root `/favicon.ico` and `manifest.json` requests with offline cache fallback.
- **Server Shutdown & Restart Cleanliness**:
  - Suppressed redundant `net.ErrClosed` (`use of closed network connection`) error logs when stopping listeners in web and sub servers during panel restarts.
- **Xray & Subscription Services Stability**:
  - Resolved panic risks and nil-pointer dereferences in subscription service generators (`subService.go`, `subJsonService.go`) and Xray API client handlers.
  - Fixed Reality fingerprint generation in subscription links to prevent invalid empty values.
  - Fixed gRPC authority handling in outbound and subscription links, properly omitting empty authority fields.
  - Fixed Hysteria protocol field formatting and added FinalMask stream crash safeguards.
- **Theme & Switcher Fixes**:
  - Fixed light/dark theme toggle bug where `#app` element theme classes were not properly synchronized upon toggle.
  - Corrected checkbox background, border contrast, checked, and indeterminate/select-all states across dark mode themes (preventing transparent blue or blinding white indeterminate states).

### 🎨 UI & Theme Polish
- Corrected selected table row background color and hover states in dark mode themes.
- Improved spacing, modal widths, and responsive layouts across Client Modal, Bulk Add Modal, and Inbound Info Modal.

### 🌐 Internationalization (i18n)
- Synced all 13 supported locale translation files (`ar_EG`, `en_US`, `es_ES`, `fa_IR`, `id_ID`, `ja_JP`, `pt_BR`, `ru_RU`, `tr_TR`, `uk_UA`, `vi_VN`, `zh_CN`, `zh_TW`), resolving missing keys such as `pages.xray.outbound.port`.

### 📦 Refactoring & CI
- Removed legacy `node_clients.html` template and obsolete backward-compatibility routes/aliases in favor of the unified `/panel/clients` architecture.
- Updated Docker CI workflow (`docker-image.yml`) with automated tagging and release pipelines.
- Bumped panel version to `0.8.6`.

---

## [v0.8.5] - 2026-10-01

### 🚀 Features & Enhancements
- **Centralized Client Management (`/panel/clients`)**:
  - Introduced a dedicated Client Management page (`/panel/clients`) consolidating multi-node client administration, search, and traffic analytics into a single responsive interface.
  - Multi-inbound client provisioning: seamlessly link a single client across multiple inbounds and protocols with synchronized usage tracking and automatic deactivation.
  - Automatic legacy migration: built-in automatic migration routine that converts legacy inbound-embedded clients into unified node clients upon accessing the clients page.
  - Client Details modal: rich modal view displaying real-time traffic statistics (up, down, total limit, remaining), expiration countdown, credentials, QR share links, and responsive auto-scaling.
  - Integrated client sorting: added ascending and descending sorting controls directly on the clients page to sort by traffic, expiry, or email/remark.
- **Xray-core v26.9.30 Feature Alignment**:
  - Upgraded core dependency to `xray-core` v26.9.30 (commit `b26a91d`) alongside updated `gRPC` and `wireguard` modules.
  - **MASQUE (RFC 9484 CONNECT-IP)**: Added comprehensive inbound, outbound, and stream transport support (`stream_masque.html`) with automatic password (`pass`) credential normalization.
  - **XDRIVE Transport**: Added support for file-based and cloud-storage-based transport stream settings (`stream_xdrive.html`).
  - **TUN Inbound Enhancements**: Added Windows Filtering Platform leak protection (`autoSystemWfpBlockLeak`) and Linux gateway DNS routing (`autoSystemDnsToGateway`).
  - **Finalmask xDNS**: Added `extraPoll` configuration parameter in stream settings for enhanced polling resilience.
  - **WireGuard Outbound Modernization**: Normalized legacy outbounds and removed obsolete `domainStrategy` from WireGuard outbound configuration.

### 🐛 Bug Fixes & Stability
- **Xray Modal Form Validation**:
  - Resolved `this.check` `ReferenceError` exception in Xray Balancer and Outbound configuration modals.
  - Added safety guard in burst observatory selector against non-array values.
- **Client IP Limit Display**:
  - Fixed fallback logic to properly display "Unlimited" across all languages when client IP limit is unset or zero.
- **Mobile Viewport Optimization**:
  - Optimized mobile layout on clients and inbounds pages with a single-row action bar, streamlined search filters, and refined table column spacing.
- **Development & Asset Paths**:
  - Corrected static filesystem asset paths to `internal/web/` for local hot-reload and debugging mode.
- **UI & Dark Mode Polish**:
  - Removed lingering hardcoded button shadows across login and dynamic theme switcher components.
  - Fixed dark mode dropdown menu titles and search icon styling.
- **PWA & Offline Reliability**:
  - Embedded base64 fallback application icon within `offline.html` and refined relative asset caching rules in `sw.js`.
- **CI / Docker**:
  - Switched container registry authentication in Docker workflow to standard `GITHUB_TOKEN`.

### 🌐 Internationalization (i18n)
- Added full translation coverage across 13 supported locales (`ar_EG`, `en_US`, `es_ES`, `fa_IR`, `id_ID`, `ja_JP`, `pt_BR`, `ru_RU`, `tr_TR`, `uk_UA`, `vi_VN`, `zh_CN`, `zh_TW`) for centralized client management, MASQUE protocol, XDRIVE transport, and TUN leak protection.

### 📦 Dependencies & Maintenance
- Updated `xray-core` (v26.9.30), `grpc`, and `wireguard` dependencies in `go.mod` and `go.sum`.
- Updated release build workflows and bumped panel version to `0.8.5`.

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
- **Xray-core v26.9.30 Upgrade**: Updated `xray-core` dependency to latest `v26.9.30` release tag across `go.mod`, `DockerInit.sh`, and GitHub Actions release workflow (`release.yml`).
- **Xray-core v26.3.27 - v26.9.30 Feature Alignment**:
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
