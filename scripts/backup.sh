#!/bin/bash
# =============================================================================
# scutum-backup.sh — Scutum database backup utility
#
# Usage:
#   ./scripts/backup.sh [output_dir]
#
# If output_dir is not specified, backups are written to ./backups/
#
# Supports:
#   - SQLite: direct file copy with WAL checkpoint
#   - PostgreSQL: pg_dump
#   - MySQL: mysqldump
#
# The backup is named:  scutum-<date>.backup[.gz]
# =============================================================================

set -euo pipefail

BACKUP_DIR="${1:-./backups}"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
mkdir -p "$BACKUP_DIR"

# --------------------------------------------------------------------------- #
# Read connection settings from environment (matching the application)
# --------------------------------------------------------------------------- #
DATABASE_URL="${DATABASE_URL:-}"
DATA_DIR="${DATA_DIR:-./data}"
SQLITE_PATH="${SQLITE_PATH:-$DATA_DIR/scutum.db}"

if [[ "$DATABASE_URL" == postgres://* ]]; then
  OUTFILE="$BACKUP_DIR/scutum-$TIMESTAMP.pgdump"
  echo "[backup] PostgreSQL → $OUTFILE"
  pg_dump --no-password --format=custom "$DATABASE_URL" -f "$OUTFILE"
  echo "[backup] Compressing..."
  gzip "$OUTFILE"
  echo "[backup] Done: ${OUTFILE}.gz"

elif [[ "$DATABASE_URL" == mysql://* ]]; then
  OUTFILE="$BACKUP_DIR/scutum-$TIMESTAMP.sql"
  echo "[backup] MySQL → $OUTFILE"
  # Strip the scheme and pass to mysqldump
  CONN="${DATABASE_URL#mysql://}"
  USER="${CONN%%:*}"
  REST="${CONN#*:}"
  PASS="${REST%%@*}"
  HOST_DB="${REST#*@}"
  HOST="${HOST_DB%%/*}"
  DBNAME="${HOST_DB#*/}"
  mysqldump -u "$USER" -p"$PASS" -h "$HOST" "$DBNAME" > "$OUTFILE"
  gzip "$OUTFILE"
  echo "[backup] Done: ${OUTFILE}.gz"

else
  # SQLite fallback
  OUTFILE="$BACKUP_DIR/scutum-$TIMESTAMP.db"
  echo "[backup] SQLite ($SQLITE_PATH) → $OUTFILE"
  if [[ ! -f "$SQLITE_PATH" ]]; then
    echo "[backup] ERROR: SQLite database not found at $SQLITE_PATH" >&2
    exit 1
  fi
  # WAL checkpoint before copy ensures a consistent snapshot
  sqlite3 "$SQLITE_PATH" "PRAGMA wal_checkpoint(FULL);" 2>/dev/null || true
  cp "$SQLITE_PATH" "$OUTFILE"
  gzip "$OUTFILE"
  echo "[backup] Done: ${OUTFILE}.gz"
fi

# --------------------------------------------------------------------------- #
# Retention: delete backups older than BACKUP_RETAIN_DAYS (default 90)
# --------------------------------------------------------------------------- #
RETAIN_DAYS="${BACKUP_RETAIN_DAYS:-90}"
echo "[backup] Purging backups older than ${RETAIN_DAYS} days from $BACKUP_DIR..."
find "$BACKUP_DIR" -name "scutum-*.gz" -mtime +"$RETAIN_DAYS" -delete
echo "[backup] Backup complete."
