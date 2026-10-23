#!/usr/bin/env python3
"""
Check, validate, and generate missing or invalid client passwords and auth tokens for x-ui / tx-ui.
Supported protocols:
  - Trojan: verifies and generates client 'password' (alphanumeric string)
  - Shadowsocks: verifies, repairs, and generates client & inbound 'password' (strict 16/32-byte Base64 key)
  - MASQUE: verifies and generates client 'password' / 'pass' (alphanumeric string)
  - Hysteria: verifies and generates client 'auth' (alphanumeric string)
  - MTProto: verifies, repairs, and generates client FakeTLS 'secret' (hex or base64 secret)
  - AmneziaWG: verifies, repairs, and generates client Curve25519 keypair ('privateKey' / 'publicKey') and PSK ('preSharedKey')
  - WireGuard: verifies, repairs, and generates client Curve25519 keypair ('privateKey' / 'publicKey') and PSK ('preSharedKey')
  - VLESS / VMess: verifies and generates client UUID ('id')

Also validates and fixes invalid Shadowsocks 2022 keys ("ErrBadKey"), MTProto secrets, and WireGuard keys,
and synchronizes with all protocol-separated columns in 'node_clients' (password, ss_password, masque_pass,
auth, secret, public_key, private_key, pre_shared_key, uuid) to prevent cross-protocol credential corruption.
"""

import argparse
import base64
import json
import os
import secrets
import shutil
import sqlite3
import string
import sys
import time
import uuid
from typing import Any, Dict, List, Optional, Set, Tuple

# Terminal colors
CLR_RESET = "\033[0m"
CLR_RED = "\033[0;31m"
CLR_GREEN = "\033[0;32m"
CLR_YELLOW = "\033[0;33m"
CLR_BLUE = "\033[0;34m"
CLR_CYAN = "\033[0;36m"
CLR_MAGENTA = "\033[0;35m"
CLR_BOLD = "\033[1m"


def log_info(msg: str) -> None:
    print(f"{CLR_GREEN}[INFO]{CLR_RESET} {msg}")


def log_warn(msg: str) -> None:
    print(f"{CLR_YELLOW}[WARN]{CLR_RESET} {msg}")


def log_error(msg: str) -> None:
    print(f"{CLR_RED}[ERROR]{CLR_RESET} {msg}")


def log_debug(msg: str) -> None:
    print(f"{CLR_CYAN}[DEBUG]{CLR_RESET} {msg}")


# ---------------------------------------------------------------------------
# Basic Credential & UUID Generators
# ---------------------------------------------------------------------------

def generate_alphanumeric(length: int = 10) -> str:
    """Generate a random alphanumeric string (matching x-ui RandomUtil.randomSeq)."""
    chars = string.ascii_letters + string.digits
    return "".join(secrets.choice(chars) for _ in range(length))


def generate_trojan_password(length: int = 10) -> str:
    """Generate a Trojan password (matching RandomUtil.randomTrojanPassword)."""
    return generate_alphanumeric(length)


def generate_masque_password(length: int = 10) -> str:
    """Generate a MASQUE password/pass token."""
    return generate_alphanumeric(length)


def generate_uuid() -> str:
    """Generate a valid UUIDv4 string."""
    return str(uuid.uuid4())


def is_valid_uuid(val: Optional[str]) -> bool:
    """Check if string is a well-formed UUID."""
    if not val or not str(val).strip():
        return False
    try:
        u = uuid.UUID(str(val).strip())
        return len(str(u)) == 36
    except Exception:
        return False


# ---------------------------------------------------------------------------
# Shadowsocks & Shadowsocks 2022 Helpers
# ---------------------------------------------------------------------------

def is_ss_2022(method: Optional[str]) -> bool:
    """Check if cipher method belongs to Shadowsocks 2022."""
    if not method:
        return False
    m = method.strip().lower()
    return m.startswith("2022")


def get_shadowsocks_expected_bytes(method: Optional[str]) -> int:
    """Return expected key byte length: 16 bytes for 128-bit, 32 bytes for 256-bit."""
    if method and "128" in method:
        return 16
    return 32


def decode_base64_flexible(s: Optional[str]) -> Optional[bytes]:
    """Decode a base64 string, restoring missing padding if needed."""
    if not s or not str(s).strip():
        return None
    clean = str(s).strip()
    padding = "=" * ((4 - len(clean) % 4) % 4)
    try:
        return base64.b64decode(clean + padding)
    except Exception:
        return None


def is_valid_shadowsocks_2022_key(key: Optional[str], method: Optional[str] = None) -> bool:
    """Validate if a Shadowsocks 2022 key is valid Base64 of the exact expected byte length."""
    raw = decode_base64_flexible(key)
    if raw is None:
        return False
    expected_len = get_shadowsocks_expected_bytes(method)
    return len(raw) == expected_len


def generate_shadowsocks_password(method: Optional[str] = None) -> str:
    """
    Generate a strict Base64 Shadowsocks key:
    - 16 bytes (Base64) for 128-bit ciphers (e.g. 2022-blake3-aes-128-gcm, aes-128-gcm)
    - 32 bytes (Base64) for 256-bit ciphers (e.g. 2022-blake3-aes-256-gcm, 2022-blake3-chacha20-poly1305, chacha20, aes-256-gcm)
    """
    expected_len = get_shadowsocks_expected_bytes(method)
    random_bytes = secrets.token_bytes(expected_len)
    return base64.b64encode(random_bytes).decode("utf-8")


# ---------------------------------------------------------------------------
# MTProto FakeTLS Secret Helpers
# ---------------------------------------------------------------------------

def is_valid_mtproto_secret(secret: Optional[str]) -> bool:
    """
    Check whether secret is a valid MTProto FakeTLS secret.
    FakeTLS secrets begin with 'ee' or 'dd' followed by at least 16 hex bytes and domain hex,
    or represent a base64-encoded secret of sufficient length (>= 17 bytes).
    """
    if not secret or not str(secret).strip():
        return False
    s = str(secret).strip()
    if s.startswith("ee") or s.startswith("dd"):
        if len(s) < 34:
            return False
        try:
            bytes.fromhex(s)
            return True
        except Exception:
            return False
    if len(s) >= 22:
        try:
            decoded = base64.b64decode(s, validate=False)
            return len(decoded) >= 17
        except Exception:
            return False
    return False


def generate_faketls_secret(domain: str = "google.com") -> str:
    """Generate an MTProto FakeTLS secret for domain (defaults to google.com)."""
    random_hex = secrets.token_hex(16)
    domain_hex = domain.encode("utf-8").hex()
    return f"ee{random_hex}{domain_hex}"


# ---------------------------------------------------------------------------
# Curve25519 / WireGuard / AmneziaWG Pure-Python Implementation (RFC 7748)
# ---------------------------------------------------------------------------

_CURVE25519_P = 2**255 - 19
_CURVE25519_A24 = 121665


def _clamp_scalar(k: bytearray) -> bytearray:
    k[0] &= 248
    k[31] &= 127
    k[31] |= 64
    return k


def _x25519(scalar_bytes: bytes, u_bytes: bytes = b"\x09" + b"\x00" * 31) -> bytes:
    """RFC 7748 Montgomery ladder X25519 scalar multiplication."""
    k = _clamp_scalar(bytearray(scalar_bytes))
    k_int = int.from_bytes(k, "little")
    u = int.from_bytes(u_bytes, "little") % _CURVE25519_P

    x_1 = u
    x_2 = 1
    z_2 = 0
    x_3 = u
    z_3 = 1
    swap = 0

    for t in reversed(range(255)):
        k_t = (k_int >> t) & 1
        swap ^= k_t
        if swap:
            x_2, x_3 = x_3, x_2
            z_2, z_3 = z_3, z_2
        swap = k_t

        A = (x_2 + z_2) % _CURVE25519_P
        AA = (A * A) % _CURVE25519_P
        B = (x_2 - z_2) % _CURVE25519_P
        BB = (B * B) % _CURVE25519_P
        E = (AA - BB) % _CURVE25519_P
        C = (x_3 + z_3) % _CURVE25519_P
        D = (x_3 - z_3) % _CURVE25519_P
        DA = (D * A) % _CURVE25519_P
        CB = (C * B) % _CURVE25519_P
        x_3 = ((DA + CB) ** 2) % _CURVE25519_P
        z_3 = (x_1 * ((DA - CB) ** 2)) % _CURVE25519_P
        x_2 = (AA * BB) % _CURVE25519_P
        z_2 = (E * (AA + _CURVE25519_A24 * E)) % _CURVE25519_P

    if swap:
        x_2, x_3 = x_3, x_2
        z_2, z_3 = z_3, z_2

    res = (x_2 * pow(z_2, _CURVE25519_P - 2, _CURVE25519_P)) % _CURVE25519_P
    return res.to_bytes(32, "little")


def decode_wireguard_key(key: Optional[str]) -> Optional[bytes]:
    """Decode a 32-byte WireGuard key from hex (64 chars) or base64 (44 chars)."""
    if not key or not str(key).strip():
        return None
    k = str(key).strip()
    if len(k) == 64:
        try:
            raw = bytes.fromhex(k)
            if len(raw) == 32:
                return raw
        except Exception:
            pass
    try:
        padding = "=" * ((4 - len(k) % 4) % 4)
        raw = base64.b64decode(k + padding, validate=False)
        if len(raw) == 32:
            return raw
    except Exception:
        pass
    return None


def is_valid_wireguard_key(key: Optional[str]) -> bool:
    """Report whether key is a valid 32-byte WireGuard/AmneziaWG key."""
    return decode_wireguard_key(key) is not None


def generate_wireguard_psk() -> str:
    """Generate a base64 encoded 32-byte pre-shared key for WireGuard/AmneziaWG."""
    return base64.b64encode(secrets.token_bytes(32)).decode("utf-8")


def generate_wireguard_keypair() -> Tuple[str, str]:
    """Generate base64 encoded Curve25519 private and public key pair for WireGuard/AmneziaWG."""
    raw_priv = bytearray(secrets.token_bytes(32))
    raw_priv[0] &= 248
    raw_priv[31] &= 127
    raw_priv[31] |= 64
    pub_bytes = _x25519(bytes(raw_priv))
    priv_b64 = base64.b64encode(bytes(raw_priv)).decode("utf-8")
    pub_b64 = base64.b64encode(pub_bytes).decode("utf-8")
    return priv_b64, pub_b64


def public_key_from_private(private_key: str) -> Optional[str]:
    """Derive the base64 public key from a valid Curve25519 private key."""
    raw = decode_wireguard_key(private_key)
    if not raw:
        return None
    pub_bytes = _x25519(raw)
    return base64.b64encode(pub_bytes).decode("utf-8")


# ---------------------------------------------------------------------------
# Database Utilities
# ---------------------------------------------------------------------------

def find_default_db() -> Optional[str]:
    """Search for the SQLite database in standard locations."""
    candidates = [
        os.path.join(os.getenv("XUI_DB_FOLDER", "/etc/x-ui"), "x-ui.db"),
        "/etc/x-ui/x-ui.db",
        "C:/etc/x-ui/x-ui.db",
        "C:\\etc\\x-ui\\x-ui.db",
        "/usr/local/x-ui/db/x-ui.db",
        "./x-ui.db",
        "./bin/x-ui.db",
    ]
    for p in candidates:
        if os.path.isfile(p):
            return os.path.abspath(p)
    return None


def table_exists(cursor: sqlite3.Cursor, table_name: str) -> bool:
    cursor.execute(
        "SELECT 1 FROM sqlite_master WHERE type='table' AND name=?",
        (table_name,),
    )
    return cursor.fetchone() is not None


def get_column_names(cursor: sqlite3.Cursor, table_name: str) -> List[str]:
    cursor.execute(f"PRAGMA table_info({table_name});")
    return [row[1] for row in cursor.fetchall()]


# ---------------------------------------------------------------------------
# Main Processing Logic
# ---------------------------------------------------------------------------

def process_database(
    db_path: str,
    dry_run: bool = False,
    no_backup: bool = False,
    fix_invalid_keys: bool = True,
) -> None:
    if not os.path.isfile(db_path):
        log_error(f"Database file not found: {db_path}")
        sys.exit(1)

    print(f"\n{CLR_BOLD}===================================================={CLR_RESET}")
    print(f"{CLR_BOLD}  tx-ui Client Password & Auth Checker / Repairer{CLR_RESET}")
    print(f"{CLR_BOLD}===================================================={CLR_RESET}")
    log_info(f"Target Database: {CLR_BOLD}{db_path}{CLR_RESET}")
    if dry_run:
        log_warn("Running in DRY-RUN mode. No changes will be saved to disk.")

    # Create backup before anything else if not dry_run and not no_backup
    if not dry_run and not no_backup:
        timestamp = time.strftime("%Y%m%d_%H%M%S")
        backup_path = f"{db_path}.backup_{timestamp}"
        try:
            shutil.copy2(db_path, backup_path)
            # Also copy -wal and -shm if they exist
            for ext in ["-wal", "-shm"]:
                wal_src = db_path + ext
                if os.path.isfile(wal_src):
                    shutil.copy2(wal_src, backup_path + ext)
            log_info(f"Database backup created: {CLR_CYAN}{backup_path}{CLR_RESET}")
        except Exception as e:
            log_error(f"Failed to create database backup: {e}")
            sys.exit(1)

    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    cursor = conn.cursor()

    # Flush WAL if present
    try:
        cursor.execute("PRAGMA wal_checkpoint(TRUNCATE);")
    except Exception:
        pass

    if not table_exists(cursor, "inbounds"):
        log_error("Table 'inbounds' not found in database. Is this a valid x-ui database?")
        conn.close()
        sys.exit(1)

    # ---------------------------------------------------------
    # 1. Process 'inbounds' table
    # ---------------------------------------------------------
    target_protocols = (
        "hysteria",
        "trojan",
        "shadowsocks",
        "masque",
        "mtproto",
        "amneziawg",
        "wireguard",
        "vless",
        "vmess",
    )
    placeholders = ",".join("?" for _ in target_protocols)
    cursor.execute(
        f"SELECT id, remark, protocol, settings, stream_settings FROM inbounds WHERE protocol IN ({placeholders})",
        target_protocols,
    )
    inbound_rows = cursor.fetchall()

    total_inbounds_checked = len(inbound_rows)
    total_clients_checked = 0
    missing_credentials_fixed = 0
    bad_keys_repaired = 0
    inbounds_to_update: List[Tuple[str, int]] = []

    # Maps to synchronize credentials from inbounds into node_clients
    email_to_auth: Dict[str, str] = {}
    email_to_trojan_pass: Dict[str, str] = {}
    email_to_ss_pass: Dict[str, str] = {}
    email_to_masque_pass: Dict[str, str] = {}
    email_to_secret: Dict[str, str] = {}
    email_to_wg_priv: Dict[str, str] = {}
    email_to_wg_pub: Dict[str, str] = {}
    email_to_wg_psk: Dict[str, str] = {}
    email_to_uuid: Dict[str, str] = {}

    print(
        f"\n{CLR_BOLD}Checking {total_inbounds_checked} inbounds "
        f"(hysteria, trojan, shadowsocks, masque, mtproto, amneziawg, wireguard, vless, vmess)...{CLR_RESET}"
    )

    for row in inbound_rows:
        inbound_id = row["id"]
        remark = row["remark"] or f"Inbound-{inbound_id}"
        protocol = (row["protocol"] or "").lower()
        settings_raw = row["settings"] or "{}"

        try:
            settings = json.loads(settings_raw)
        except json.JSONDecodeError:
            log_warn(f"Skipping Inbound [{inbound_id}] '{remark}': invalid JSON in settings")
            continue

        inbound_modified = False
        inbound_method = settings.get("method") or ""

        # Check single-user shadowsocks top-level password
        if protocol == "shadowsocks":
            top_pass = settings.get("password")
            is_empty = top_pass is None or str(top_pass).strip() == ""
            is_2022 = is_ss_2022(inbound_method) or not inbound_method

            if is_empty:
                new_pass = generate_shadowsocks_password(inbound_method)
                settings["password"] = new_pass
                inbound_modified = True
                missing_credentials_fixed += 1
                log_info(
                    f"Inbound [{inbound_id}] '{remark}' (Shadowsocks): generated missing top-level password -> {CLR_YELLOW}{new_pass}{CLR_RESET}"
                )
            elif is_2022 and fix_invalid_keys and not is_valid_shadowsocks_2022_key(top_pass, inbound_method):
                new_pass = generate_shadowsocks_password(inbound_method)
                settings["password"] = new_pass
                inbound_modified = True
                bad_keys_repaired += 1
                log_warn(
                    f"Inbound [{inbound_id}] '{remark}' (Shadowsocks 2022): repaired bad/invalid top-level key '{top_pass}' -> {CLR_GREEN}{new_pass}{CLR_RESET}"
                )

        # Check AmneziaWG / WireGuard server keypair if present
        if protocol in ("amneziawg", "wireguard"):
            server_block = settings.get("server")
            if isinstance(server_block, dict):
                s_priv = server_block.get("privateKey")
                s_pub = server_block.get("publicKey")
                if not is_valid_wireguard_key(s_priv):
                    new_priv, new_pub = generate_wireguard_keypair()
                    server_block["privateKey"] = new_priv
                    server_block["publicKey"] = new_pub
                    inbound_modified = True
                    if not s_priv:
                        missing_credentials_fixed += 1
                        log_info(f"Inbound [{inbound_id}] '{remark}' ({protocol}): generated server keypair")
                    else:
                        bad_keys_repaired += 1
                        log_warn(f"Inbound [{inbound_id}] '{remark}' ({protocol}): repaired invalid server keypair")
                elif not is_valid_wireguard_key(s_pub):
                    derived_pub = public_key_from_private(str(s_priv)) or generate_wireguard_keypair()[1]
                    server_block["publicKey"] = derived_pub
                    inbound_modified = True
                    bad_keys_repaired += 1
                    log_warn(f"Inbound [{inbound_id}] '{remark}' ({protocol}): derived missing/invalid server publicKey")

        # Resolve clients list (or peers for wireguard)
        clients = settings.get("clients")
        peers_used = False
        if clients is None and "peers" in settings:
            clients = settings.get("peers")
            peers_used = True
        if not isinstance(clients, list):
            clients = []

        for idx, client in enumerate(clients):
            if not isinstance(client, dict):
                continue
            total_clients_checked += 1
            email = str(client.get("email") or f"client_{idx+1}")

            if protocol == "hysteria":
                current_auth = client.get("auth")
                if current_auth is None or str(current_auth).strip() == "":
                    if client.get("password") and str(client.get("password")).strip():
                        new_auth = str(client.get("password")).strip()
                        action_type = "copied from password"
                    else:
                        new_auth = generate_alphanumeric(10)
                        action_type = "generated"

                    client["auth"] = new_auth
                    inbound_modified = True
                    missing_credentials_fixed += 1
                    email_to_auth[email] = new_auth
                    log_info(
                        f"Inbound [{inbound_id}] '{remark}' (Hysteria) Client '{email}': "
                        f"{action_type} missing auth -> {CLR_YELLOW}{new_auth}{CLR_RESET}"
                    )
                else:
                    email_to_auth[email] = str(current_auth).strip()

            elif protocol == "trojan":
                current_pass = client.get("password")
                if current_pass is None or str(current_pass).strip() == "":
                    new_pass = generate_trojan_password(10)
                    client["password"] = new_pass
                    inbound_modified = True
                    missing_credentials_fixed += 1
                    email_to_trojan_pass[email] = new_pass
                    log_info(
                        f"Inbound [{inbound_id}] '{remark}' (Trojan) Client '{email}': "
                        f"generated missing password -> {CLR_YELLOW}{new_pass}{CLR_RESET}"
                    )
                else:
                    email_to_trojan_pass[email] = str(current_pass).strip()

            elif protocol == "shadowsocks":
                current_pass = client.get("password")
                client_method = client.get("method") or inbound_method
                is_2022 = is_ss_2022(client_method) or is_ss_2022(inbound_method) or not client_method
                is_empty = current_pass is None or str(current_pass).strip() == ""

                if is_empty:
                    new_pass = generate_shadowsocks_password(client_method)
                    client["password"] = new_pass
                    inbound_modified = True
                    missing_credentials_fixed += 1
                    email_to_ss_pass[email] = new_pass
                    log_info(
                        f"Inbound [{inbound_id}] '{remark}' (Shadowsocks) Client '{email}': "
                        f"generated missing key (method: {client_method or '2022-blake3-aes-256-gcm'}) -> {CLR_YELLOW}{new_pass}{CLR_RESET}"
                    )
                elif is_2022 and fix_invalid_keys and not is_valid_shadowsocks_2022_key(current_pass, client_method):
                    new_pass = generate_shadowsocks_password(client_method)
                    client["password"] = new_pass
                    inbound_modified = True
                    bad_keys_repaired += 1
                    email_to_ss_pass[email] = new_pass
                    log_warn(
                        f"Inbound [{inbound_id}] '{remark}' (Shadowsocks 2022) Client '{email}': "
                        f"repaired bad key '{current_pass}' (ErrBadKey fix) -> {CLR_GREEN}{new_pass}{CLR_RESET}"
                    )
                else:
                    email_to_ss_pass[email] = str(current_pass).strip()

            elif protocol == "masque":
                current_pass = client.get("password") or client.get("pass")
                if current_pass is None or str(current_pass).strip() == "":
                    new_pass = generate_masque_password(10)
                    client["password"] = new_pass
                    client["pass"] = new_pass
                    inbound_modified = True
                    missing_credentials_fixed += 1
                    email_to_masque_pass[email] = new_pass
                    log_info(
                        f"Inbound [{inbound_id}] '{remark}' (MASQUE) Client '{email}': "
                        f"generated missing password -> {CLR_YELLOW}{new_pass}{CLR_RESET}"
                    )
                else:
                    clean_pass = str(current_pass).strip()
                    if client.get("password") != clean_pass or client.get("pass") != clean_pass:
                        client["password"] = clean_pass
                        client["pass"] = clean_pass
                        inbound_modified = True
                    email_to_masque_pass[email] = clean_pass

            elif protocol == "mtproto":
                current_secret = client.get("secret") or client.get("password") or client.get("id")
                if current_secret is None or str(current_secret).strip() == "":
                    new_secret = generate_faketls_secret()
                    client["secret"] = new_secret
                    client["password"] = new_secret
                    client["id"] = new_secret
                    inbound_modified = True
                    missing_credentials_fixed += 1
                    email_to_secret[email] = new_secret
                    log_info(
                        f"Inbound [{inbound_id}] '{remark}' (MTProto) Client '{email}': "
                        f"generated missing FakeTLS secret -> {CLR_YELLOW}{new_secret}{CLR_RESET}"
                    )
                elif not is_valid_mtproto_secret(current_secret):
                    if fix_invalid_keys:
                        new_secret = generate_faketls_secret()
                        client["secret"] = new_secret
                        client["password"] = new_secret
                        client["id"] = new_secret
                        inbound_modified = True
                        bad_keys_repaired += 1
                        email_to_secret[email] = new_secret
                        log_warn(
                            f"Inbound [{inbound_id}] '{remark}' (MTProto) Client '{email}': "
                            f"repaired invalid secret '{current_secret}' -> {CLR_GREEN}{new_secret}{CLR_RESET}"
                        )
                    else:
                        email_to_secret[email] = str(current_secret).strip()
                else:
                    clean_secret = str(current_secret).strip()
                    if (
                        client.get("secret") != clean_secret
                        or client.get("password") != clean_secret
                        or client.get("id") != clean_secret
                    ):
                        client["secret"] = clean_secret
                        client["password"] = clean_secret
                        client["id"] = clean_secret
                        inbound_modified = True
                    email_to_secret[email] = clean_secret

            elif protocol in ("wireguard", "amneziawg"):
                priv = client.get("privateKey")
                pub = client.get("publicKey") or client.get("id")
                psk = client.get("preSharedKey") or client.get("psk")

                # Validate or generate Curve25519 keypair
                if not is_valid_wireguard_key(priv):
                    new_priv, new_pub = generate_wireguard_keypair()
                    client["privateKey"] = new_priv
                    client["publicKey"] = new_pub
                    client["id"] = new_pub
                    client["password"] = new_priv
                    inbound_modified = True
                    if not priv or not str(priv).strip():
                        missing_credentials_fixed += 1
                        log_info(
                            f"Inbound [{inbound_id}] '{remark}' ({protocol}) Client '{email}': "
                            f"generated missing Curve25519 keypair -> {CLR_YELLOW}{new_pub}{CLR_RESET}"
                        )
                    else:
                        bad_keys_repaired += 1
                        log_warn(
                            f"Inbound [{inbound_id}] '{remark}' ({protocol}) Client '{email}': "
                            f"repaired bad Curve25519 key '{priv}' -> {CLR_GREEN}{new_pub}{CLR_RESET}"
                        )
                    email_to_wg_priv[email] = new_priv
                    email_to_wg_pub[email] = new_pub
                else:
                    clean_priv = str(priv).strip()
                    client["privateKey"] = clean_priv
                    client["password"] = clean_priv
                    email_to_wg_priv[email] = clean_priv

                    if not is_valid_wireguard_key(pub):
                        derived_pub = public_key_from_private(clean_priv) or generate_wireguard_keypair()[1]
                        client["publicKey"] = derived_pub
                        client["id"] = derived_pub
                        inbound_modified = True
                        if not pub or not str(pub).strip():
                            missing_credentials_fixed += 1
                            log_info(
                                f"Inbound [{inbound_id}] '{remark}' ({protocol}) Client '{email}': "
                                f"derived missing publicKey -> {CLR_YELLOW}{derived_pub}{CLR_RESET}"
                            )
                        else:
                            bad_keys_repaired += 1
                            log_warn(
                                f"Inbound [{inbound_id}] '{remark}' ({protocol}) Client '{email}': "
                                f"repaired bad publicKey '{pub}' -> {CLR_GREEN}{derived_pub}{CLR_RESET}"
                            )
                        email_to_wg_pub[email] = derived_pub
                    else:
                        clean_pub = str(pub).strip()
                        client["publicKey"] = clean_pub
                        client["id"] = clean_pub
                        email_to_wg_pub[email] = clean_pub

                # Validate or generate PreSharedKey (PSK)
                if psk and not is_valid_wireguard_key(psk):
                    if fix_invalid_keys:
                        new_psk = generate_wireguard_psk()
                        client["preSharedKey"] = new_psk
                        client["psk"] = new_psk
                        inbound_modified = True
                        bad_keys_repaired += 1
                        email_to_wg_psk[email] = new_psk
                        log_warn(
                            f"Inbound [{inbound_id}] '{remark}' ({protocol}) Client '{email}': "
                            f"repaired bad PSK -> {CLR_GREEN}{new_psk}{CLR_RESET}"
                        )
                    else:
                        email_to_wg_psk[email] = str(psk).strip()
                elif psk:
                    clean_psk = str(psk).strip()
                    client["preSharedKey"] = clean_psk
                    client["psk"] = clean_psk
                    email_to_wg_psk[email] = clean_psk
                elif protocol == "amneziawg":
                    # AmneziaWG clients default to having a dedicated PSK
                    new_psk = generate_wireguard_psk()
                    client["preSharedKey"] = new_psk
                    client["psk"] = new_psk
                    inbound_modified = True
                    missing_credentials_fixed += 1
                    email_to_wg_psk[email] = new_psk
                    log_info(
                        f"Inbound [{inbound_id}] '{remark}' (AmneziaWG) Client '{email}': "
                        f"generated missing PSK -> {CLR_YELLOW}{new_psk}{CLR_RESET}"
                    )

            elif protocol in ("vless", "vmess"):
                cid = client.get("id")
                if not cid or not str(cid).strip():
                    new_uuid = generate_uuid()
                    client["id"] = new_uuid
                    inbound_modified = True
                    missing_credentials_fixed += 1
                    email_to_uuid[email] = new_uuid
                    log_info(
                        f"Inbound [{inbound_id}] '{remark}' ({protocol.upper()}) Client '{email}': "
                        f"generated missing UUID -> {CLR_YELLOW}{new_uuid}{CLR_RESET}"
                    )
                elif not is_valid_uuid(cid):
                    if fix_invalid_keys:
                        new_uuid = generate_uuid()
                        client["id"] = new_uuid
                        inbound_modified = True
                        bad_keys_repaired += 1
                        email_to_uuid[email] = new_uuid
                        log_warn(
                            f"Inbound [{inbound_id}] '{remark}' ({protocol.upper()}) Client '{email}': "
                            f"repaired bad UUID '{cid}' -> {CLR_GREEN}{new_uuid}{CLR_RESET}"
                        )
                    else:
                        email_to_uuid[email] = str(cid).strip()
                else:
                    email_to_uuid[email] = str(cid).strip()

        if inbound_modified:
            if peers_used and "peers" in settings:
                settings["peers"] = clients
            else:
                settings["clients"] = clients
            new_settings_str = json.dumps(settings, indent=2, ensure_ascii=False)
            inbounds_to_update.append((new_settings_str, inbound_id))

    # ---------------------------------------------------------
    # 2. Process 'node_clients' table if present
    # ---------------------------------------------------------
    has_node_clients = table_exists(cursor, "node_clients")
    has_node_links = table_exists(cursor, "node_client_links")
    fixed_node_clients_count = 0
    repaired_node_clients_count = 0
    node_clients_updates: List[Tuple[str, List[Any], int]] = []

    if has_node_clients:
        nc_cols = get_column_names(cursor, "node_clients")
        has_pass_col = "password" in nc_cols
        has_ss_pass_col = "ss_password" in nc_cols
        has_masque_pass_col = "masque_pass" in nc_cols
        has_auth_col = "auth" in nc_cols
        has_secret_col = "secret" in nc_cols
        has_pub_col = "public_key" in nc_cols
        has_priv_col = "private_key" in nc_cols
        has_psk_col = "pre_shared_key" in nc_cols
        has_uuid_col = "uuid" in nc_cols

        print(
            f"\n{CLR_BOLD}Checking 'node_clients' table for missing credentials and bad keys "
            f"(with decoupled protocol columns)...{CLR_RESET}"
        )

        # Map node_client_id -> protocols of linked inbounds & methods
        client_linked_protocols: Dict[int, Set[str]] = {}
        client_ss_methods: Dict[int, List[str]] = {}
        if has_node_links:
            cursor.execute("""
                SELECT l.node_client_id, i.protocol, i.settings
                FROM node_client_links l
                JOIN inbounds i ON i.id = l.inbound_id
            """)
            for link_row in cursor.fetchall():
                nc_id = link_row[0]
                proto = (link_row[1] or "").lower()
                settings_text = link_row[2] or "{}"
                client_linked_protocols.setdefault(nc_id, set()).add(proto)
                if proto == "shadowsocks":
                    try:
                        s = json.loads(settings_text)
                        m = s.get("method") or ""
                        client_ss_methods.setdefault(nc_id, []).append(m)
                    except Exception:
                        pass

        cursor.execute("SELECT * FROM node_clients")
        node_clients = cursor.fetchall()

        for nc in node_clients:
            nc_id = nc["id"]
            nc_email = nc["email"] or ""
            linked_protos = client_linked_protocols.get(nc_id, set())
            updates: Dict[str, Any] = {}

            # --- Trojan / General password ---
            nc_pass = nc["password"] if has_pass_col else None
            is_trojan = "trojan" in linked_protos or nc_email in email_to_trojan_pass
            if has_pass_col:
                if is_trojan or not linked_protos:
                    if nc_pass is None or str(nc_pass).strip() == "":
                        new_trojan = email_to_trojan_pass.get(nc_email) or generate_trojan_password(10)
                        updates["password"] = new_trojan
                        fixed_node_clients_count += 1
                        log_info(
                            f"NodeClient [{nc_id}] '{nc_email}': generated missing Trojan password -> {CLR_YELLOW}{new_trojan}{CLR_RESET}"
                        )
                elif not has_ss_pass_col and "shadowsocks" in linked_protos:
                    # Legacy fallback when ss_password column does not exist
                    ss_methods = client_ss_methods.get(nc_id, [])
                    ss_method = ss_methods[0] if ss_methods else ""
                    if nc_pass is None or str(nc_pass).strip() == "":
                        new_pass = email_to_ss_pass.get(nc_email) or generate_shadowsocks_password(ss_method)
                        updates["password"] = new_pass
                        fixed_node_clients_count += 1
                    elif fix_invalid_keys and not is_valid_shadowsocks_2022_key(nc_pass, ss_method):
                        new_pass = email_to_ss_pass.get(nc_email) or generate_shadowsocks_password(ss_method)
                        updates["password"] = new_pass
                        repaired_node_clients_count += 1

            # --- Shadowsocks (ss_password) ---
            if has_ss_pass_col:
                nc_ss = nc["ss_password"]
                is_ss = "shadowsocks" in linked_protos or nc_email in email_to_ss_pass
                ss_methods = client_ss_methods.get(nc_id, [])
                ss_method = ss_methods[0] if ss_methods else ""
                is_2022 = is_ss_2022(ss_method) or not ss_method

                if is_ss:
                    if nc_ss is None or str(nc_ss).strip() == "":
                        new_ss = email_to_ss_pass.get(nc_email) or generate_shadowsocks_password(ss_method)
                        updates["ss_password"] = new_ss
                        fixed_node_clients_count += 1
                        log_info(
                            f"NodeClient [{nc_id}] '{nc_email}': synchronized/generated missing Shadowsocks key -> {CLR_YELLOW}{new_ss}{CLR_RESET}"
                        )
                    elif is_2022 and fix_invalid_keys and not is_valid_shadowsocks_2022_key(nc_ss, ss_method):
                        new_ss = (
                            email_to_ss_pass.get(nc_email)
                            if is_valid_shadowsocks_2022_key(email_to_ss_pass.get(nc_email), ss_method)
                            else generate_shadowsocks_password(ss_method)
                        )
                        updates["ss_password"] = new_ss
                        repaired_node_clients_count += 1
                        log_warn(
                            f"NodeClient [{nc_id}] '{nc_email}': repaired bad Shadowsocks 2022 key '{nc_ss}' -> {CLR_GREEN}{new_ss}{CLR_RESET}"
                        )
                elif nc_ss and fix_invalid_keys and not is_valid_shadowsocks_2022_key(nc_ss, ss_method):
                    new_ss = generate_shadowsocks_password(ss_method)
                    updates["ss_password"] = new_ss
                    repaired_node_clients_count += 1
                    log_warn(
                        f"NodeClient [{nc_id}] '{nc_email}': repaired bad Shadowsocks 2022 key '{nc_ss}' -> {CLR_GREEN}{new_ss}{CLR_RESET}"
                    )

            # --- MASQUE (masque_pass) ---
            if has_masque_pass_col:
                nc_masque = nc["masque_pass"]
                is_masque = "masque" in linked_protos or nc_email in email_to_masque_pass
                if is_masque and (nc_masque is None or str(nc_masque).strip() == ""):
                    new_masque = email_to_masque_pass.get(nc_email) or generate_masque_password(10)
                    updates["masque_pass"] = new_masque
                    fixed_node_clients_count += 1
                    log_info(
                        f"NodeClient [{nc_id}] '{nc_email}': synchronized/generated missing MASQUE password -> {CLR_YELLOW}{new_masque}{CLR_RESET}"
                    )

            # --- Hysteria (auth) ---
            if has_auth_col:
                nc_auth = nc["auth"]
                is_hysteria = "hysteria" in linked_protos or nc_email in email_to_auth
                if is_hysteria and (nc_auth is None or str(nc_auth).strip() == ""):
                    if nc_email in email_to_auth:
                        new_auth = email_to_auth[nc_email]
                    elif nc_pass and str(nc_pass).strip():
                        new_auth = str(nc_pass).strip()
                    else:
                        new_auth = generate_alphanumeric(10)
                    updates["auth"] = new_auth
                    fixed_node_clients_count += 1
                    log_info(
                        f"NodeClient [{nc_id}] '{nc_email}': updated missing auth -> {CLR_YELLOW}{new_auth}{CLR_RESET}"
                    )

            # --- MTProto (secret) ---
            if has_secret_col:
                nc_secret = nc["secret"]
                is_mtproto = "mtproto" in linked_protos or nc_email in email_to_secret
                if is_mtproto:
                    if nc_secret is None or str(nc_secret).strip() == "":
                        new_sec = email_to_secret.get(nc_email) or generate_faketls_secret()
                        updates["secret"] = new_sec
                        fixed_node_clients_count += 1
                        log_info(
                            f"NodeClient [{nc_id}] '{nc_email}': synchronized/generated missing MTProto secret -> {CLR_YELLOW}{new_sec}{CLR_RESET}"
                        )
                    elif not is_valid_mtproto_secret(nc_secret):
                        if fix_invalid_keys:
                            new_sec = (
                                email_to_secret.get(nc_email)
                                if is_valid_mtproto_secret(email_to_secret.get(nc_email))
                                else generate_faketls_secret()
                            )
                            updates["secret"] = new_sec
                            repaired_node_clients_count += 1
                            log_warn(
                                f"NodeClient [{nc_id}] '{nc_email}': repaired invalid MTProto secret '{nc_secret}' -> {CLR_GREEN}{new_sec}{CLR_RESET}"
                            )
                elif nc_secret and not is_valid_mtproto_secret(nc_secret):
                    if fix_invalid_keys:
                        new_sec = generate_faketls_secret()
                        updates["secret"] = new_sec
                        repaired_node_clients_count += 1
                        log_warn(
                            f"NodeClient [{nc_id}] '{nc_email}': repaired invalid MTProto secret '{nc_secret}' -> {CLR_GREEN}{new_sec}{CLR_RESET}"
                        )

            # --- WireGuard & AmneziaWG (public_key, private_key, pre_shared_key) ---
            is_wg_or_awg = bool(linked_protos.intersection({"wireguard", "amneziawg"})) or (
                nc_email in email_to_wg_pub or nc_email in email_to_wg_priv
            )
            if has_priv_col and has_pub_col:
                nc_priv = nc["private_key"]
                nc_pub = nc["public_key"]

                if is_wg_or_awg:
                    if not is_valid_wireguard_key(nc_priv):
                        if nc_email in email_to_wg_priv and is_valid_wireguard_key(email_to_wg_priv[nc_email]):
                            new_priv = email_to_wg_priv[nc_email]
                            new_pub = email_to_wg_pub.get(nc_email) or public_key_from_private(new_priv)
                        else:
                            new_priv, new_pub = generate_wireguard_keypair()
                        updates["private_key"] = new_priv
                        updates["public_key"] = new_pub
                        if not nc_priv or not str(nc_priv).strip():
                            fixed_node_clients_count += 1
                            log_info(
                                f"NodeClient [{nc_id}] '{nc_email}': synchronized/generated WireGuard keypair -> {CLR_YELLOW}{new_pub}{CLR_RESET}"
                            )
                        else:
                            repaired_node_clients_count += 1
                            log_warn(
                                f"NodeClient [{nc_id}] '{nc_email}': repaired bad WireGuard privateKey '{nc_priv}' -> {CLR_GREEN}{new_pub}{CLR_RESET}"
                            )
                    elif not is_valid_wireguard_key(nc_pub):
                        derived_pub = (
                            email_to_wg_pub.get(nc_email)
                            or public_key_from_private(str(nc_priv))
                            or generate_wireguard_keypair()[1]
                        )
                        updates["public_key"] = derived_pub
                        if not nc_pub or not str(nc_pub).strip():
                            fixed_node_clients_count += 1
                            log_info(
                                f"NodeClient [{nc_id}] '{nc_email}': derived missing WireGuard publicKey -> {CLR_YELLOW}{derived_pub}{CLR_RESET}"
                            )
                        else:
                            repaired_node_clients_count += 1
                            log_warn(
                                f"NodeClient [{nc_id}] '{nc_email}': repaired bad WireGuard publicKey '{nc_pub}' -> {CLR_GREEN}{derived_pub}{CLR_RESET}"
                            )
                elif nc_priv or nc_pub:
                    # Clean up orphaned invalid keys if present
                    if not is_valid_wireguard_key(nc_priv) and fix_invalid_keys:
                        new_priv, new_pub = generate_wireguard_keypair()
                        updates["private_key"] = new_priv
                        updates["public_key"] = new_pub
                        repaired_node_clients_count += 1
                    elif not is_valid_wireguard_key(nc_pub) and fix_invalid_keys:
                        derived_pub = public_key_from_private(str(nc_priv))
                        if derived_pub:
                            updates["public_key"] = derived_pub
                            repaired_node_clients_count += 1

            if has_psk_col:
                nc_psk = nc["pre_shared_key"]
                if is_wg_or_awg:
                    if nc_psk is None or str(nc_psk).strip() == "":
                        if nc_email in email_to_wg_psk:
                            new_psk = email_to_wg_psk[nc_email]
                            updates["pre_shared_key"] = new_psk
                            fixed_node_clients_count += 1
                            log_info(
                                f"NodeClient [{nc_id}] '{nc_email}': synchronized WireGuard PSK -> {CLR_YELLOW}{new_psk}{CLR_RESET}"
                            )
                    elif not is_valid_wireguard_key(nc_psk) and fix_invalid_keys:
                        new_psk = (
                            email_to_wg_psk.get(nc_email)
                            if is_valid_wireguard_key(email_to_wg_psk.get(nc_email))
                            else generate_wireguard_psk()
                        )
                        updates["pre_shared_key"] = new_psk
                        repaired_node_clients_count += 1
                        log_warn(
                            f"NodeClient [{nc_id}] '{nc_email}': repaired bad WireGuard PSK -> {CLR_GREEN}{new_psk}{CLR_RESET}"
                        )
                elif nc_psk and not is_valid_wireguard_key(nc_psk) and fix_invalid_keys:
                    new_psk = generate_wireguard_psk()
                    updates["pre_shared_key"] = new_psk
                    repaired_node_clients_count += 1

            # --- VMess / VLESS (uuid) ---
            if has_uuid_col:
                nc_uuid = nc["uuid"]
                is_vless_or_vmess = bool(linked_protos.intersection({"vless", "vmess"})) or nc_email in email_to_uuid
                if is_vless_or_vmess:
                    if not nc_uuid or not str(nc_uuid).strip():
                        new_uuid = email_to_uuid.get(nc_email) or generate_uuid()
                        updates["uuid"] = new_uuid
                        fixed_node_clients_count += 1
                        log_info(
                            f"NodeClient [{nc_id}] '{nc_email}': synchronized/generated missing UUID -> {CLR_YELLOW}{new_uuid}{CLR_RESET}"
                        )
                    elif not is_valid_uuid(nc_uuid) and fix_invalid_keys:
                        new_uuid = (
                            email_to_uuid.get(nc_email)
                            if is_valid_uuid(email_to_uuid.get(nc_email))
                            else generate_uuid()
                        )
                        updates["uuid"] = new_uuid
                        repaired_node_clients_count += 1
                        log_warn(
                            f"NodeClient [{nc_id}] '{nc_email}': repaired bad UUID '{nc_uuid}' -> {CLR_GREEN}{new_uuid}{CLR_RESET}"
                        )
                elif not nc_uuid or not str(nc_uuid).strip():
                    new_uuid = generate_uuid()
                    updates["uuid"] = new_uuid
                    fixed_node_clients_count += 1
                elif not is_valid_uuid(nc_uuid) and fix_invalid_keys:
                    new_uuid = generate_uuid()
                    updates["uuid"] = new_uuid
                    repaired_node_clients_count += 1

            if updates:
                set_clauses = [f"{col} = ?" for col in updates.keys()]
                vals = list(updates.values())
                sql = f"UPDATE node_clients SET {', '.join(set_clauses)} WHERE id = ?"
                node_clients_updates.append((sql, vals, nc_id))

    # ---------------------------------------------------------
    # 3. Apply changes (if not dry_run)
    # ---------------------------------------------------------
    if not dry_run:
        if inbounds_to_update or node_clients_updates:
            try:
                cursor.execute("BEGIN IMMEDIATE;")
                for new_settings, ib_id in inbounds_to_update:
                    cursor.execute("UPDATE inbounds SET settings = ? WHERE id = ?", (new_settings, ib_id))

                for sql, vals, nc_id in node_clients_updates:
                    cursor.execute(sql, vals + [nc_id])

                conn.commit()
                cursor.execute("PRAGMA wal_checkpoint(TRUNCATE);")
                log_info(f"{CLR_GREEN}Database successfully updated!{CLR_RESET}")
            except Exception as e:
                conn.rollback()
                log_error(f"Transaction failed and was rolled back: {e}")
                conn.close()
                sys.exit(1)
        else:
            log_info("No missing passwords or invalid keys found. Database is completely up to date.")
    else:
        if inbounds_to_update or node_clients_updates:
            log_warn("Dry-run complete: no changes were written to the database.")
        else:
            log_info("Dry-run complete: all clients have valid passwords and auth.")

    conn.close()

    # ---------------------------------------------------------
    # 4. Summary report
    # ---------------------------------------------------------
    print(f"\n{CLR_BOLD}---------------- Summary ----------------{CLR_RESET}")
    print(f"Total inbounds scanned           : {total_inbounds_checked}")
    print(f"Total clients scanned            : {total_clients_checked}")
    print(f"Missing client credentials fixed : {CLR_BOLD}{missing_credentials_fixed}{CLR_RESET}")
    print(f"Bad / invalid keys repaired      : {CLR_BOLD}{bad_keys_repaired}{CLR_RESET}")
    if has_node_clients:
        print(f"Node clients missing fixed       : {CLR_BOLD}{fixed_node_clients_count}{CLR_RESET}")
        print(f"Node clients bad keys repaired   : {CLR_BOLD}{repaired_node_clients_count}{CLR_RESET}")
    total_modifications = (
        missing_credentials_fixed + bad_keys_repaired + fixed_node_clients_count + repaired_node_clients_count
    )
    print(f"Total fixes applied              : {CLR_BOLD}{total_modifications}{CLR_RESET}")
    print(f"{CLR_BOLD}-----------------------------------------{CLR_RESET}\n")

    if total_modifications > 0 and not dry_run:
        print(f"{CLR_YELLOW}{CLR_BOLD}NOTICE:{CLR_RESET} Please restart your x-ui panel service to apply the new credentials:")
        print(f"  {CLR_CYAN}systemctl restart x-ui{CLR_RESET}   (or: {CLR_CYAN}x-ui restart{CLR_RESET})\n")


def main():
    parser = argparse.ArgumentParser(
        description="Verify, generate, and repair client credentials (Trojan, Shadowsocks 2022, MASQUE, Hysteria, MTProto, AmneziaWG, WireGuard, VLESS/VMess) in x-ui / tx-ui."
    )
    parser.add_argument(
        "--db",
        dest="db_path",
        default=None,
        help="Path to x-ui SQLite database file (e.g. /etc/x-ui/x-ui.db). If omitted, auto-detects.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Check and print missing or invalid credentials without modifying the database.",
    )
    parser.add_argument(
        "--no-backup",
        action="store_true",
        help="Do not create a database backup file before making changes.",
    )
    parser.add_argument(
        "--no-fix-invalid-keys",
        dest="fix_invalid_keys",
        action="store_false",
        default=True,
        help="Do not repair invalid/non-Base64 keys (only fill missing passwords).",
    )

    args = parser.parse_args()

    db_path = args.db_path or find_default_db()
    if not db_path:
        log_error("Could not auto-detect x-ui.db location.")
        print("Please specify the database path using the --db option, for example:")
        print("  python3 fix_client_passwords.py --db /etc/x-ui/x-ui.db\n")
        sys.exit(1)

    process_database(
        db_path,
        dry_run=args.dry_run,
        no_backup=args.no_backup,
        fix_invalid_keys=args.fix_invalid_keys,
    )


if __name__ == "__main__":
    main()
