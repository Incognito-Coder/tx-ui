# TX-UI API Documentation

This document describes the HTTP REST API exposed by **TX-UI**. All endpoints return JSON responses wrapped in a standard result structure.

---

## Table of Contents

- [Overview & Architecture](#overview--architecture)
- [Authentication & Sessions](#authentication--sessions)
- [Standard Response Format](#standard-response-format)
- [1. Authentication Endpoints](#1-authentication-endpoints)
- [2. Clients API (Node Clients)](#2-clients-api-node-clients)
- [3. Inbounds API](#3-inbounds-api)
- [4. Server & System API](#4-server--system-api)
- [5. Settings & User Management](#5-settings--user-management)
- [6. Xray Configuration API](#6-xray-configuration-api)
- [7. Code Examples](#7-code-examples)
  - [cURL](#curl-example)
  - [Python](#python-example)

---

## Overview & Architecture

- **Base Path:** Typically `/` or custom subpath configured via `webBasePath` (default: `/`).
- **API Base:** `<base_path>panel/api/`
- **Protocol:** HTTP or HTTPS depending on panel configuration.
- **Content-Type:** `application/json` or `application/x-www-form-urlencoded` for POST requests.
- **Session Management:** Cookie-based session (`tx-ui` cookie).

---

## Authentication & Sessions

1. Authenticate using `POST /login` with username, password, and optional two-factor login secret.
2. The server sets a `Set-Cookie: tx-ui=...; Path=/; HttpOnly` header on success.
3. Include this cookie in all subsequent API requests.
4. If unauthenticated, requests to `/panel/api/*` return `404 Not Found` for security obscurity.

---

## Standard Response Format

All JSON API responses conform to the following schema:

```json
{
  "success": true,
  "msg": "Operation successful",
  "obj": {}
}
```

- **`success`** (`boolean`): Indicates whether the request succeeded (`true`) or failed (`false`).
- **`msg`** (`string`): Human-readable message or localized status message.
- **`obj`** (`any`): The returned payload (object, array, number, string, or `null`).

---

## 1. Authentication Endpoints

### 1.1 Login
Authenticate and establish a session.

- **Method:** `POST`
- **Path:** `/login`
- **Content-Type:** `application/json` or `application/x-www-form-urlencoded`
- **Request Body:**
  ```json
  {
    "username": "admin",
    "password": "your_password",
    "loginSecret": ""
  }
  ```
- **Response:**
  ```json
  {
    "success": true,
    "msg": "Login successful"
  }
  ```

### 1.2 Logout
Terminate the current session.

- **Method:** `GET`
- **Path:** `/logout`
- **Response:** 302 Redirect to `/login` or 200 OK.

### 1.3 Check Two-Factor Secret Status
Check whether a two-factor login secret is configured.

- **Method:** `POST`
- **Path:** `/getSecretStatus`
- **Response:**
  ```json
  {
    "success": true,
    "msg": "",
    "obj": false
  }
  ```

---

## 2. Clients API (Node Clients)

Primary API for managing global client identities across inbounds.
- **Base Route:** `/panel/api/clients`

### 2.1 List All Clients
Retrieve all clients with traffic, status, and linked inbound details.

- **Method:** `GET`
- **Path:** `/panel/api/clients/list`
- **Response `obj`:** Array of Client objects:
  ```json
  {
    "success": true,
    "obj": [
      {
        "id": 1,
        "email": "user@example.com",
        "subId": "sub_random123",
        "uuid": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
        "password": "secret_password",
        "auth": "auth_token",
        "flow": "xtls-rprx-vision",
        "totalGB": 107374182400,
        "expiryTime": 1775000000000,
        "limitIp": 2,
        "tgId": 123456789,
        "enable": true,
        "reset": 30,
        "comment": "VIP user",
        "up": 1048576,
        "down": 10485760,
        "total": 11534336,
        "inboundLinks": [
          {
            "id": 1,
            "nodeClientId": 1,
            "inboundId": 1,
            "flow": ""
          }
        ]
      }
    ]
  }
  ```

### 2.2 Get Single Client
Retrieve details of a specific client by ID.

- **Method:** `GET`
- **Path:** `/panel/api/clients/get/:id`
- **Parameters:**
  - `id` (`integer`, path): Client ID.

### 2.3 Create Client
Create a new client identity and optionally link it to inbounds.

- **Method:** `POST`
- **Path:** `/panel/api/clients/create`
- **Request Body:**
  ```json
  {
    "email": "newuser@example.com",
    "subId": "custom_sub_id",
    "uuid": "auto_or_custom_uuid",
    "password": "optional_password",
    "flow": "xtls-rprx-vision",
    "totalGB": 53687091200,
    "expiryTime": 0,
    "limitIp": 1,
    "tgId": 0,
    "enable": true,
    "reset": 0,
    "comment": "Created via API",
    "inboundIds": [1, 2]
  }
  ```
  *(Note: `totalGB` is passed in bytes. E.g., `50 * 1024 * 1024 * 1024 = 53687091200` for 50 GB. Set `0` for unlimited).*

### 2.4 Bulk Create Clients
Generate multiple clients at once.

- **Method:** `POST`
- **Path:** `/panel/api/clients/bulkCreate`
- **Request Body:**
  ```json
  {
    "count": 5,
    "prefix": "client_",
    "totalGB": 21474836480,
    "expiryTime": 0,
    "limitIp": 0,
    "flow": "",
    "inboundIds": [1]
  }
  ```

### 2.5 Update Client
Update an existing client's settings.

- **Method:** `POST`
- **Path:** `/panel/api/clients/update/:id`
- **Parameters:**
  - `id` (`integer`, path): Client ID.
- **Request Body:** Same fields as create (e.g. `email`, `totalGB`, `expiryTime`, `enable`, `limitIp`, `comment`, `inboundIds`).

### 2.6 Delete Client
Permanently delete a client and its associated links.

- **Method:** `POST`
- **Path:** `/panel/api/clients/del/:id`
- **Parameters:**
  - `id` (`integer`, path): Client ID.

### 2.7 Bulk Delete Clients
Delete multiple clients simultaneously.

- **Method:** `POST`
- **Path:** `/panel/api/clients/bulkDel`
- **Request Body:**
  ```json
  {
    "ids": [1, 2, 3]
  }
  ```

### 2.8 Toggle Client Status
Enable or disable a client.

- **Method:** `POST`
- **Path:** `/panel/api/clients/:id/toggle`
- **Parameters:**
  - `id` (`integer`, path): Client ID.
- **Request Body (optional):**
  ```json
  {
    "enable": false
  }
  ```

### 2.9 Client Links Management
- **`GET /panel/api/clients/:id/links`**: Get list of inbound IDs linked to this client.
- **`POST /panel/api/clients/:id/addLink`**: Add inbound link (`inboundId`, `flow`).
- **`POST /panel/api/clients/:id/setLinks`**: Replace all inbound links (`links: [{ inboundId: 1, flow: "" }]`).
- **`POST /panel/api/clients/:id/removeLink/:inboundId`**: Remove link between client and an inbound.

### 2.10 Client Traffic Management
- **`GET /panel/api/clients/:id/traffic`**: Get real-time upload/download bytes.
- **`POST /panel/api/clients/:id/resetTraffic`**: Reset traffic statistics for a client to 0.
- **`POST /panel/api/clients/resetAllTraffics`**: Reset traffic statistics for all clients.
- **`POST /panel/api/clients/delDepleted`**: Delete all clients whose traffic or expiration date is depleted.
- **`GET /panel/api/clients/inboundLinkCounts`**: Get count of clients linked to each inbound.

---

## 3. Inbounds API

Manage listening ports, protocols, and stream configurations.
- **Base Route:** `/panel/api/inbounds`

### 3.1 List Inbounds
Retrieve all configured inbounds.

- **Method:** `GET`
- **Path:** `/panel/api/inbounds/list`
- **Response `obj`:** Array of Inbound objects including port, protocol, settings, streamSettings, sniffing, and traffic.

### 3.2 Get Inbound
Get a single inbound by ID.

- **Method:** `GET`
- **Path:** `/panel/api/inbounds/get/:id`

### 3.3 Add Inbound
Create a new proxy inbound.

- **Method:** `POST`
- **Path:** `/panel/api/inbounds/add`
- **Request Body:**
  ```json
  {
    "enable": true,
    "remark": "VLESS-Reality",
    "listen": "0.0.0.0",
    "port": 443,
    "protocol": "vless",
    "expiryTime": 0,
    "settings": "{\"clients\":[],\"decryption\":\"none\",\"fallbacks\":[]}",
    "streamSettings": "{\"network\":\"tcp\",\"security\":\"reality\",\"realitySettings\":{...}}",
    "sniffing": "{\"enabled\":true,\"destOverride\":[\"http\",\"tls\",\"quic\"]}"
  }
  ```

### 3.4 Update Inbound
Update an existing inbound configuration.

- **Method:** `POST`
- **Path:** `/panel/api/inbounds/update/:id`

### 3.5 Delete Inbound
Delete an inbound and its associated clients.

- **Method:** `POST`
- **Path:** `/panel/api/inbounds/del/:id`

### 3.6 Reorder Inbounds
Update the display sort order of inbounds.

- **Method:** `POST`
- **Path:** `/panel/api/inbounds/reorder`
- **Request Body:**
  ```json
  [
    { "id": 1, "sort": 0 },
    { "id": 2, "sort": 1 }
  ]
  ```

### 3.7 Inbound Client Traffic & IP Management
- **`GET /panel/api/inbounds/getClientTraffics/:email`**: Get client traffic stats by email.
- **`GET /panel/api/inbounds/getClientTrafficsById/:id`**: Get client traffic stats by ID.
- **`POST /panel/api/inbounds/updateClientTraffic/:email`**: Adjust client upload/download traffic in bytes (`upload`, `download`).
- **`POST /panel/api/inbounds/clientIps/:email`**: Get recorded IP addresses for a client.
- **`POST /panel/api/inbounds/clearClientIps/:email`**: Clear recorded IP addresses for a client.
- **`POST /panel/api/inbounds/:id/resetClientTraffic/:email`**: Reset a client's traffic counters.
- **`POST /panel/api/inbounds/resetAllTraffics`**: Reset traffic for all inbounds.
- **`POST /panel/api/inbounds/resetAllClientTraffics/:id`**: Reset all client traffic for an inbound.
- **`POST /panel/api/inbounds/delDepletedClients/:id`**: Delete expired/depleted clients from an inbound (`-1` for all inbounds).
- **`POST /panel/api/inbounds/onlines`**: Get list of currently online client emails.
- **`POST /panel/api/inbounds/depleted`**: Get list of depleted client emails.
- **`POST /panel/api/inbounds/disabled`**: Get list of disabled client emails.

---

## 4. Server & System API

Query server status, manage Xray core, and execute maintenance operations.
- **Base Route:** `/panel/api/server`

### 4.1 Server Status
Retrieve CPU, memory, disk, network, and uptime metrics.

- **Method:** `GET`
- **Path:** `/panel/api/server/status`
- **Response `obj`:**
  ```json
  {
    "cpu": 12.5,
    "mem": { "current": 1073741824, "total": 4294967296 },
    "swap": { "current": 0, "total": 2147483648 },
    "disk": { "current": 10737418240, "total": 53687091200 },
    "xray": { "state": "running", "errorMsg": "", "version": "v24.9.30" },
    "uptime": 86400,
    "loads": [0.15, 0.22, 0.18],
    "tcpCount": 42,
    "udpCount": 18,
    "netIO": { "up": 1048576, "down": 4194304 },
    "netTraffic": { "sent": 10737418240, "recv": 53687091200 }
  }
  ```

### 4.2 Xray Core Controls
- **`POST /panel/api/server/restartXrayService`**: Restart the Xray core service.
- **`POST /panel/api/server/stopXrayService`**: Stop the Xray core service.
- **`GET /panel/api/server/getXrayVersion`**: Get installed Xray version.
- **`GET /panel/api/server/getXrayMetrics`**: Get internal Prometheus metrics from Xray.
- **`GET /panel/api/server/getConfigJson`**: Retrieve the generated `config.json` running in Xray.
- **`POST /panel/api/server/installXray/:version`**: Download and install a specific Xray core release.

### 4.3 Cryptographic Tools
- **`GET /panel/api/server/getNewUUID`**: Generate a new random UUID v4.
- **`GET /panel/api/server/getNewX25519Cert`**: Generate a Reality X25519 keypair (`privateKey`, `publicKey`).
- **`GET /panel/api/server/getNewCert`**: Generate a self-signed TLS certificate and private key.
- **`GET /panel/api/server/getNewmldsa65`**: Generate ML-DSA-65 post-quantum keypair.
- **`GET /panel/api/server/getNewmlkem768`**: Generate ML-KEM-768 post-quantum keypair.
- **`GET /panel/api/server/getNewVlessEnc`**: Generate VLESS encryption keys.
- **`POST /panel/api/server/getNewEchCert`**: Generate ECH (Encrypted Client Hello) keys and configs.

### 4.4 Maintenance & Logs
- **`GET /panel/api/server/getDb`**: Download SQLite database file (`x-ui.db`).
- **`POST /panel/api/server/importDB`**: Restore database from an uploaded `.db` file (multipart form `db`).
- **`POST /panel/api/server/logs/:count`**: Fetch the last `:count` lines of TX-UI panel logs.
- **`POST /panel/api/server/xraylogs/:count`**: Fetch the last `:count` lines of Xray core logs.
- **`GET /panel/api/server/getGeoVersions`**: Check current GeoIP and GeoSite versions.
- **`POST /panel/api/server/updateGeoFiles`**: Update GeoIP and GeoSite databases to latest version.
- **`GET /panel/api/server/getSubTemplates`**: List available subscription templates.
- **`POST /panel/api/server/applySubTemplate`**: Apply a subscription template from a GitHub URL (`url`).
- **`POST /panel/api/server/resetSubTemplate`**: Reset subscription template to default.
- **`GET /panel/api/backuptotgbot`**: Trigger an immediate Telegram database backup to configured admins.

---

## 5. Settings & User Management

- **Base Route:** `/panel/setting`

- **`POST /panel/setting/all`**: Get all panel settings (key-value dictionary).
- **`POST /panel/setting/update`**: Update panel settings by posting key-value form fields.
- **`POST /panel/setting/defaultSettings`**: Reset all settings to factory defaults.
- **`POST /panel/setting/updateUser`**: Change panel admin credentials (`oldUsername`, `oldPassword`, `newUsername`, `newPassword`).
- **`POST /panel/setting/restartPanel`**: Gracefully restart the TX-UI web process.
- **`GET /panel/setting/getDefaultJsonConfig`**: Retrieve default template config.
- **`POST /panel/setting/getUserSecret`**: Get 2FA QR code and secret URI.
- **`POST /panel/setting/updateUserSecret`**: Enable or disable 2FA (`secret`).

---

## 6. Xray Configuration API

- **Base Route:** `/panel/xray`

- **`POST /panel/xray/`**: Retrieve complete Xray advanced configuration (routing, DNS, balancers, outbounds).
- **`POST /panel/xray/update`**: Save and apply advanced Xray JSON configuration.
- **`GET /panel/xray/getXrayResult`**: Validate current configuration and return test results.
- **`POST /panel/xray/testOutbound`**: Test outbound connectivity and latency (`outbound`, optional `allOutbounds`).
- **`GET /panel/xray/getOutboundsTraffic`**: Get cumulative traffic per outbound tag.
- **`POST /panel/xray/resetOutboundsTraffic`**: Reset outbound traffic counters.
- **`POST /panel/xray/warp/:action`**: Control Cloudflare WARP integration (`reg`, `del`, etc.).

---

## 7. Code Examples

### cURL Example

```bash
# 1. Login and save session cookie
curl -s -c cookies.txt -X POST "http://YOUR_SERVER:2087/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your_password"}'

# 2. Get server status
curl -s -b cookies.txt "http://YOUR_SERVER:2087/panel/api/server/status"

# 3. List all clients
curl -s -b cookies.txt "http://YOUR_SERVER:2087/panel/api/clients/list"

# 4. Create a new client
curl -s -b cookies.txt -X POST "http://YOUR_SERVER:2087/panel/api/clients/create" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user1@example.com",
    "totalGB": 53687091200,
    "expiryTime": 0,
    "enable": true,
    "inboundIds": [1]
  }'

# 5. Restart Xray core
curl -s -b cookies.txt -X POST "http://YOUR_SERVER:2087/panel/api/server/restartXrayService"
```

### Python Example

```python
import requests

BASE_URL = "http://YOUR_SERVER:2087"
USERNAME = "admin"
PASSWORD = "your_password"

session = requests.Session()

# 1. Login
login_resp = session.post(f"{BASE_URL}/login", json={
    "username": USERNAME,
    "password": PASSWORD
})
login_data = login_resp.json()
if not login_data.get("success"):
    raise Exception(f"Login failed: {login_data.get('msg')}")
print("Logged in successfully!")

# 2. Fetch all clients
clients_resp = session.get(f"{BASE_URL}/panel/api/clients/list")
clients = clients_resp.json().get("obj", [])
print(f"Found {len(clients)} clients:")
for c in clients:
    print(f"  - [{c.get('id')}] {c.get('email')} (Enabled: {c.get('enable')})")

# 3. Create a client with 50 GB traffic limit
new_client_payload = {
    "email": "py_client@domain.com",
    "totalGB": 50 * 1024 * 1024 * 1024, # 50 GB in bytes
    "expiryTime": 0,                    # 0 = indefinite
    "limitIp": 2,
    "enable": True,
    "inboundIds": [1]                   # Link to Inbound #1
}
create_resp = session.post(f"{BASE_URL}/panel/api/clients/create", json=new_client_payload)
print("Create client response:", create_resp.json())

# 4. Check Server System Status
status_resp = session.get(f"{BASE_URL}/panel/api/server/status")
status = status_resp.json().get("obj", {})
print("Xray State:", status.get("xray", {}).get("state"))
print("CPU Usage:", status.get("cpu"), "%")
print("Memory Used:", status.get("mem", {}).get("current"), "/", status.get("mem", {}).get("total"))
```
