#!/usr/bin/env python3
"""
Check and generate missing client passwords and auth tokens for x-ui / tx-ui.
Supported protocols:
  - Hysteria: verifies and generates client 'auth'
  - Trojan: verifies and generates client 'password'
  - Shadowsocks: verifies and generates client 'password' (16/32-byte base64 key)

Also synchronizes with the 'node_clients' table if present in the database.
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
from typing import Any, Dict, List, Optional, Tuple

# Terminal colors
CLR_RESET = "\033[0m"
CLR_RED = "\033[0;31m"
CLR_GREEN = "\033[0;32m"
CLR_YELLOW = "\033[0;33m"
CLR_BLUE = "\033[0;34m"
CLR_CYAN = "\033[0;36m"
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


def generate_shadowsocks_password(method: Optional[str] = None) -> str:
    """
    Generate a Shadowsocks key.
    - 16 bytes base64 for 128-bit ciphers (e.g. 2022-blake3-aes-128-gcm, aes-128-gcm)
    - 32 bytes base64 for 256-bit ciphers (e.g. 2022-blake3-aes-256-gcm, chacha20, aes-256-gcm)
    """
    if method and "128" in method:
        random_bytes = secrets.token_bytes(16)
    else:
        random_bytes = secrets.token_bytes(32)
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


def process_database(db_path: str, dry_run: bool = False, no_backup: bool = False) -> None:
    if not os.path.isfile(db_path):
        log_error(f"Database file not found: {db_path}")
        sys.exit(1)

    print(f"\n{CLR_BOLD}===================================================={CLR_RESET}")
    print(f"{CLR_BOLD}  x-ui / tx-ui Client Password & Auth Checker{CLR_RESET}")
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
        "SELECT id, remark, protocol, settings, stream_settings FROM inbounds WHERE protocol IN ('hysteria', 'trojan', 'shadowsocks')"
    )
    inbound_rows = cursor.fetchall()

    total_inbounds_checked = len(inbound_rows)
    total_clients_checked = 0
    fixed_clients_count = 0
    inbounds_to_update: List[Tuple[str, int]] = []

    # Map email -> generated/verified credentials to keep node_clients synchronized
    email_to_auth: Dict[str, str] = {}
    email_to_password: Dict[str, str] = {}

    print(f"\n{CLR_BOLD}Checking {total_inbounds_checked} inbounds (hysteria, trojan, shadowsocks)...{CLR_RESET}")

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

        inbound_method = settings.get("method")
        clients = settings.get("clients", [])
        if not isinstance(clients, list):
            continue

        inbound_modified = False

        # Also check single-user shadowsocks / trojan top-level password
        if protocol == "shadowsocks":
            top_pass = settings.get("password")
            if (top_pass is None or str(top_pass).strip() == "") and (not clients):
                new_pass = generate_shadowsocks_password(inbound_method)
                settings["password"] = new_pass
                inbound_modified = True
                fixed_clients_count += 1
                log_info(
                    f"Inbound [{inbound_id}] '{remark}' (Shadowsocks): generated missing top-level password -> {new_pass}"
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
                    fixed_clients_count += 1
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
                    new_pass = generate_alphanumeric(10)
                    client["password"] = new_pass
                    inbound_modified = True
                    fixed_clients_count += 1
                    email_to_password[email] = new_pass
                    log_info(
                        f"Inbound [{inbound_id}] '{remark}' (Trojan) Client '{email}': "
                        f"generated missing password -> {CLR_YELLOW}{new_pass}{CLR_RESET}"
                    )
                else:
                    email_to_password[email] = str(current_pass).strip()

            elif protocol == "shadowsocks":
                current_pass = client.get("password")
                client_method = client.get("method") or inbound_method
                if current_pass is None or str(current_pass).strip() == "":
                    new_pass = generate_shadowsocks_password(client_method)
                    client["password"] = new_pass
                    inbound_modified = True
                    fixed_clients_count += 1
                    email_to_password[email] = new_pass
                    log_info(
                        f"Inbound [{inbound_id}] '{remark}' (Shadowsocks) Client '{email}': "
                        f"generated missing password (method: {client_method or 'default'}) -> {CLR_YELLOW}{new_pass}{CLR_RESET}"
                    )
                else:
                    email_to_password[email] = str(current_pass).strip()

        if inbound_modified:
            new_settings_str = json.dumps(settings, indent=2, ensure_ascii=False)
            inbounds_to_update.append((new_settings_str, inbound_id))

    # ---------------------------------------------------------
    # 2. Process 'node_clients' table if present
    # ---------------------------------------------------------
    has_node_clients = table_exists(cursor, "node_clients")
    has_node_links = table_exists(cursor, "node_client_links")
    fixed_node_clients_count = 0
    node_clients_to_update: List[Tuple[Any, ...]] = []

    if has_node_clients:
        nc_cols = get_column_names(cursor, "node_clients")
        has_auth_col = "auth" in nc_cols
        has_pass_col = "password" in nc_cols

        print(f"\n{CLR_BOLD}Checking 'node_clients' table for missing credentials...{CLR_RESET}")

        # Map node_client_id -> protocols of linked inbounds
        client_linked_protocols: Dict[int, set] = {}
        if has_node_links:
            cursor.execute("""
                SELECT l.node_client_id, i.protocol
                FROM node_client_links l
                JOIN inbounds i ON i.id = l.inbound_id
            """)
            for link_row in cursor.fetchall():
                nc_id = link_row[0]
                proto = (link_row[1] or "").lower()
                client_linked_protocols.setdefault(nc_id, set()).add(proto)

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
                    log_info(
                        f"NodeClient [{nc_id}] '{nc_email}': updated missing auth -> {CLR_YELLOW}{new_auth}{CLR_RESET}"
                    )

            # Check if linked to Trojan / Shadowsocks or email matched
            is_trojan_or_ss = (
                bool(linked_protos.intersection({"trojan", "shadowsocks"}))
                or nc_email in email_to_password
            )
            if has_pass_col and is_trojan_or_ss:
                if updated_pass is None or str(updated_pass).strip() == "":
                    if nc_email in email_to_password:
                        new_pass = email_to_password[nc_email]
                    elif "shadowsocks" in linked_protos:
                        new_pass = generate_shadowsocks_password()
                    else:
                        new_pass = generate_alphanumeric(10)
                    updated_pass = new_pass
                    nc_modified = True
                    log_info(
                        f"NodeClient [{nc_id}] '{nc_email}': updated missing password -> {CLR_YELLOW}{new_pass}{CLR_RESET}"
                    )

            if nc_modified:
                fixed_node_clients_count += 1
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
            log_info("No missing passwords or auth found. Database is completely up to date.")
    else:
        if inbounds_to_update or node_clients_to_update:
            log_warn("Dry-run complete: no changes were written to the database.")
        else:
            log_info("Dry-run complete: all clients already have passwords/auth.")

    conn.close()

    # ---------------------------------------------------------
    # 4. Summary report
    # ---------------------------------------------------------
    print(f"\n{CLR_BOLD}---------------- Summary ----------------{CLR_RESET}")
    print(f"Total inbounds scanned       : {total_inbounds_checked}")
    print(f"Total clients scanned        : {total_clients_checked}")
    print(f"Missing client credentials fixed : {CLR_BOLD}{fixed_clients_count}{CLR_RESET}")
    if has_node_clients:
        print(f"Node clients updated         : {CLR_BOLD}{fixed_node_clients_count}{CLR_RESET}")
    print(f"{CLR_BOLD}-----------------------------------------{CLR_RESET}\n")

    if (fixed_clients_count > 0 or fixed_node_clients_count > 0) and not dry_run:
        print(f"{CLR_YELLOW}{CLR_BOLD}NOTICE:{CLR_RESET} Please restart your x-ui panel service to apply the new credentials:")
        print(f"  {CLR_CYAN}systemctl restart x-ui{CLR_RESET}   (or: {CLR_CYAN}x-ui restart{CLR_RESET})\n")


def main():
    parser = argparse.ArgumentParser(
        description="Verify and generate missing Hysteria auth, Trojan password, and Shadowsocks password in x-ui / tx-ui."
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
        help="Check and print missing credentials without modifying the database.",
    )
    parser.add_argument(
        "--no-backup",
        action="store_true",
        help="Do not create a database backup file before making changes.",
    )

    args = parser.parse_args()

    db_path = args.db_path or find_default_db()
    if not db_path:
        log_error("Could not auto-detect x-ui.db location.")
        print("Please specify the database path using the --db option, for example:")
        print("  python3 fix_client_passwords.py --db /etc/x-ui/x-ui.db\n")
        sys.exit(1)

    process_database(db_path, dry_run=args.dry_run, no_backup=args.no_backup)


if __name__ == "__main__":
    main()
