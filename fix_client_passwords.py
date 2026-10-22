#!/usr/bin/env python3
"""
Check, validate, and generate missing or invalid client passwords and auth tokens for x-ui / tx-ui.
Supported protocols:
  - Hysteria: verifies and generates client 'auth'
  - Trojan: verifies and generates client 'password' (alphanumeric string)
  - Shadowsocks: verifies and generates client 'password' (strict 16/32-byte Base64 key)
  - MASQUE: verifies and generates client 'password' / 'pass'

Also validates and fixes invalid Shadowsocks 2022 keys ("ErrBadKey"),
and synchronizes with the 'node_clients' table if present in the database.
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


def is_valid_shadowsocks_2022_key(key: Optional[str], method: Optional[str] = None) -> bool:
    """Validate if a Shadowsocks 2022 key is valid Base64 of the exact expected byte length."""
    if not key or not str(key).strip():
        return False
    expected_len = get_shadowsocks_expected_bytes(method)
    try:
        decoded = base64.b64decode(str(key).strip(), validate=True)
        return len(decoded) == expected_len
    except Exception:
        return False


def generate_shadowsocks_password(method: Optional[str] = None) -> str:
    """
    Generate a strict Base64 Shadowsocks key:
    - 16 bytes (Base64) for 128-bit ciphers (e.g. 2022-blake3-aes-128-gcm, aes-128-gcm)
    - 32 bytes (Base64) for 256-bit ciphers (e.g. 2022-blake3-aes-256-gcm, 2022-blake3-chacha20-poly1305, chacha20, aes-256-gcm)
    """
    expected_len = get_shadowsocks_expected_bytes(method)
    random_bytes = secrets.token_bytes(expected_len)
    return base64.b64encode(random_bytes).decode("utf-8")


def find_default_db() -> Optional[str]:
    """Search for the SQLite database in standard locations."""
    candidates = [
        os.path.join(os.getenv("XUI_DB_FOLDER", "/etc/x-ui"), "x-ui.db"),
        "/etc/x-ui/x-ui.db",
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
    cursor.execute(
        "SELECT id, remark, protocol, settings, stream_settings FROM inbounds WHERE protocol IN ('hysteria', 'trojan', 'shadowsocks', 'masque')"
    )
    inbound_rows = cursor.fetchall()

    total_inbounds_checked = len(inbound_rows)
    total_clients_checked = 0
    missing_credentials_fixed = 0
    bad_keys_repaired = 0
    inbounds_to_update: List[Tuple[str, int]] = []

    # Map email -> generated/verified credentials to keep node_clients synchronized
    email_to_auth: Dict[str, str] = {}
    email_to_trojan_pass: Dict[str, str] = {}
    email_to_ss_pass: Dict[str, str] = {}
    email_to_masque_pass: Dict[str, str] = {}

    print(f"\n{CLR_BOLD}Checking {total_inbounds_checked} inbounds (hysteria, trojan, shadowsocks, masque)...{CLR_RESET}")

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

        inbound_method = settings.get("method") or ""
        clients = settings.get("clients", [])
        if not isinstance(clients, list):
            clients = []

        inbound_modified = False

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

        for idx, client in enumerate(clients):
            if not isinstance(client, dict):
                continue
            total_clients_checked += 1
            email = str(client.get("email") or f"client_{idx+1}")

            if protocol == "hysteria":
                current_auth = client.get("auth")
                if current_auth is None or str(current_auth).strip() == "":
                    # Check if password field exists and is non-empty
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

        if inbound_modified:
            new_settings_str = json.dumps(settings, indent=2, ensure_ascii=False)
            inbounds_to_update.append((new_settings_str, inbound_id))

    # ---------------------------------------------------------
    # 2. Process 'node_clients' table if present
    # ---------------------------------------------------------
    has_node_clients = table_exists(cursor, "node_clients")
    has_node_links = table_exists(cursor, "node_client_links")
    fixed_node_clients_count = 0
    repaired_node_clients_count = 0
    node_clients_to_update: List[Tuple[Any, ...]] = []

    if has_node_clients:
        nc_cols = get_column_names(cursor, "node_clients")
        has_auth_col = "auth" in nc_cols
        has_pass_col = "password" in nc_cols

        print(f"\n{CLR_BOLD}Checking 'node_clients' table for missing credentials and bad keys...{CLR_RESET}")

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

        cursor.execute("SELECT id, email, password, auth FROM node_clients")
        node_clients = cursor.fetchall()

        for nc in node_clients:
            nc_id = nc["id"]
            nc_email = nc["email"] or ""
            nc_pass = nc["password"]
            nc_auth = nc["auth"] if has_auth_col else None
            linked_protos = client_linked_protocols.get(nc_id, set())

            updated_pass = nc_pass
            updated_auth = nc_auth
            nc_modified = False

            # Check if linked to Hysteria or if email matched Hysteria inbound
            is_hysteria = "hysteria" in linked_protos or nc_email in email_to_auth
            if has_auth_col and is_hysteria:
                if updated_auth is None or str(updated_auth).strip() == "":
                    if nc_email in email_to_auth:
                        new_auth = email_to_auth[nc_email]
                    elif updated_pass and str(updated_pass).strip():
                        new_auth = str(updated_pass).strip()
                    else:
                        new_auth = generate_alphanumeric(10)
                    updated_auth = new_auth
                    nc_modified = True
                    fixed_node_clients_count += 1
                    log_info(
                        f"NodeClient [{nc_id}] '{nc_email}': updated missing auth -> {CLR_YELLOW}{new_auth}{CLR_RESET}"
                    )

            # Check if linked to Shadowsocks, Trojan, or MASQUE
            is_ss = "shadowsocks" in linked_protos or nc_email in email_to_ss_pass
            is_trojan = "trojan" in linked_protos or nc_email in email_to_trojan_pass
            is_masque = "masque" in linked_protos or nc_email in email_to_masque_pass

            if has_pass_col and (is_ss or is_trojan or is_masque or not linked_protos):
                is_pass_empty = updated_pass is None or str(updated_pass).strip() == ""

                if is_ss:
                    # Determine SS method
                    ss_methods = client_ss_methods.get(nc_id, [])
                    ss_method = ss_methods[0] if ss_methods else ""
                    is_2022 = is_ss_2022(ss_method) or not ss_method

                    if is_pass_empty:
                        new_pass = email_to_ss_pass.get(nc_email) or generate_shadowsocks_password(ss_method)
                        updated_pass = new_pass
                        nc_modified = True
                        fixed_node_clients_count += 1
                        log_info(
                            f"NodeClient [{nc_id}] '{nc_email}': generated missing Shadowsocks key -> {CLR_YELLOW}{new_pass}{CLR_RESET}"
                        )
                    elif is_2022 and fix_invalid_keys and not is_valid_shadowsocks_2022_key(updated_pass, ss_method):
                        new_pass = email_to_ss_pass.get(nc_email) or generate_shadowsocks_password(ss_method)
                        updated_pass = new_pass
                        nc_modified = True
                        repaired_node_clients_count += 1
                        log_warn(
                            f"NodeClient [{nc_id}] '{nc_email}': repaired bad Shadowsocks 2022 key '{nc_pass}' -> {CLR_GREEN}{new_pass}{CLR_RESET}"
                        )
                elif is_pass_empty:
                    if is_trojan:
                        new_pass = email_to_trojan_pass.get(nc_email) or generate_trojan_password(10)
                        label = "Trojan password"
                    elif is_masque:
                        new_pass = email_to_masque_pass.get(nc_email) or generate_masque_password(10)
                        label = "MASQUE password"
                    else:
                        new_pass = generate_trojan_password(10)
                        label = "default client password"

                    updated_pass = new_pass
                    nc_modified = True
                    fixed_node_clients_count += 1
                    log_info(
                        f"NodeClient [{nc_id}] '{nc_email}': generated missing {label} -> {CLR_YELLOW}{new_pass}{CLR_RESET}"
                    )

            if nc_modified:
                if has_auth_col and has_pass_col:
                    node_clients_to_update.append((updated_pass, updated_auth, nc_id))
                elif has_pass_col:
                    node_clients_to_update.append((updated_pass, nc_id))

    # ---------------------------------------------------------
    # 3. Apply changes (if not dry_run)
    # ---------------------------------------------------------
    if not dry_run:
        if inbounds_to_update or node_clients_to_update:
            try:
                cursor.execute("BEGIN IMMEDIATE;")
                for new_settings, ib_id in inbounds_to_update:
                    cursor.execute("UPDATE inbounds SET settings = ? WHERE id = ?", (new_settings, ib_id))

                if has_node_clients and node_clients_to_update:
                    nc_cols = get_column_names(cursor, "node_clients")
                    if "auth" in nc_cols and "password" in nc_cols:
                        cursor.executemany(
                            "UPDATE node_clients SET password = ?, auth = ? WHERE id = ?",
                            node_clients_to_update,
                        )
                    elif "password" in nc_cols:
                        cursor.executemany(
                            "UPDATE node_clients SET password = ? WHERE id = ?",
                            node_clients_to_update,
                        )

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
        if inbounds_to_update or node_clients_to_update:
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
    print(f"Bad Shadowsocks keys repaired    : {CLR_BOLD}{bad_keys_repaired}{CLR_RESET}")
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
        description="Verify, generate, and repair client passwords (Trojan, Shadowsocks 2022, MASQUE) and Hysteria auth in x-ui / tx-ui."
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
        help="Do not repair invalid/non-Base64 Shadowsocks 2022 keys (only fill missing passwords).",
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
