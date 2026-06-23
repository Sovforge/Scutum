package handlers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupStore is the subset of the store needed by BackupHandler.
type BackupStore interface {
	WalCheckpoint(ctx context.Context) error
}

// BackupHandler manages on-demand database backups.
// Backups are written to <dataDir>/backups/ as gzipped files.
type BackupHandler struct {
	store      BackupStore
	dataDir    string
	driver     string // "sqlite" | "postgres" | "mysql"
	dbURL      string // postgres/mysql DSN; empty for sqlite
	sqlitePath string // full path to the sqlite file
}

func NewBackupHandler(store BackupStore, dataDir, driver, dbURL, sqlitePath string) *BackupHandler {
	return &BackupHandler{
		store:      store,
		dataDir:    dataDir,
		driver:     driver,
		dbURL:      dbURL,
		sqlitePath: sqlitePath,
	}
}

// BackupRecord describes a single backup file.
type BackupRecord struct {
	ID        string `json:"id"`        // filename (url-safe)
	Filename  string `json:"filename"`
	Driver    string `json:"driver"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

func (h *BackupHandler) backupDir() string {
	return filepath.Join(h.dataDir, "backups")
}

// HandleList returns all backup files sorted newest-first.
func (h *BackupHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	dir := h.backupDir()
	if err := os.MkdirAll(dir, 0750); err != nil {
		http.Error(w, `{"error":"cannot access backup directory"}`, http.StatusInternalServerError)
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, `{"error":"cannot read backup directory"}`, http.StatusInternalServerError)
		return
	}

	var records []BackupRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "scutum-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		records = append(records, BackupRecord{
			ID:        e.Name(),
			Filename:  e.Name(),
			Driver:    driverFromFilename(e.Name()),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}

	// newest first
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt > records[j].CreatedAt
	})
	if records == nil {
		records = []BackupRecord{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// HandleCreate triggers an on-demand backup and returns the new BackupRecord.
func (h *BackupHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	dir := h.backupDir()
	if err := os.MkdirAll(dir, 0750); err != nil {
		http.Error(w, `{"error":"cannot create backup directory"}`, http.StatusInternalServerError)
		return
	}

	ts := time.Now().UTC().Format("20060102_150405")
	var filename string
	var runErr error

	switch h.driver {
	case "postgres":
		filename = fmt.Sprintf("scutum-%s.pgdump.gz", ts)
		runErr = h.backupPostgres(filepath.Join(dir, filename))
	case "mysql":
		filename = fmt.Sprintf("scutum-%s.sql.gz", ts)
		runErr = h.backupMySQL(filepath.Join(dir, filename))
	default:
		filename = fmt.Sprintf("scutum-%s.db.gz", ts)
		runErr = h.backupSQLite(r.Context(), filepath.Join(dir, filename))
	}

	if runErr != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, runErr.Error()), http.StatusInternalServerError)
		return
	}

	info, _ := os.Stat(filepath.Join(dir, filename))
	record := BackupRecord{
		ID:        filename,
		Filename:  filename,
		Driver:    h.driver,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if info != nil {
		record.SizeBytes = info.Size()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

// HandleDownload streams the gzipped backup file to the client.
func (h *BackupHandler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	if !isValidBackupName(name) {
		http.Error(w, `{"error":"invalid backup id"}`, http.StatusBadRequest)
		return
	}
	path := filepath.Join(h.backupDir(), name)
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, `{"error":"backup not found"}`, http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Type", "application/gzip")
	io.Copy(w, f) //nolint:errcheck
}

// HandleDelete removes a backup file.
func (h *BackupHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	if !isValidBackupName(name) {
		http.Error(w, `{"error":"invalid backup id"}`, http.StatusBadRequest)
		return
	}
	path := filepath.Join(h.backupDir(), name)
	if err := os.Remove(path); err != nil {
		http.Error(w, `{"error":"backup not found or already deleted"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// HandleRestore restores a backup in-place.
// For SQLite this replaces the live database file — a restart is required.
// For PostgreSQL/MySQL external tooling (psql/mysql) must be available.
func (h *BackupHandler) HandleRestore(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("id")
	if !isValidBackupName(name) {
		http.Error(w, `{"error":"invalid backup id"}`, http.StatusBadRequest)
		return
	}
	path := filepath.Join(h.backupDir(), name)
	if _, err := os.Stat(path); err != nil {
		http.Error(w, `{"error":"backup not found"}`, http.StatusNotFound)
		return
	}

	var restoreErr error
	switch h.driver {
	case "postgres":
		restoreErr = h.restorePostgres(path)
	case "mysql":
		restoreErr = h.restoreMySQL(path)
	default:
		restoreErr = h.restoreSQLite(path)
	}

	if restoreErr != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, restoreErr.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "restored",
		"requires_restart": h.driver == "sqlite",
		"message":          restoreMessage(h.driver),
	})
}

// ── Backup implementations ────────────────────────────────────────────────────

func (h *BackupHandler) backupSQLite(ctx context.Context, dest string) error {
	if h.sqlitePath == "" {
		return fmt.Errorf("sqlite path not configured")
	}
	// WAL checkpoint before copy for a consistent snapshot
	if h.store != nil {
		_ = h.store.WalCheckpoint(ctx)
	}

	src, err := os.Open(h.sqlitePath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer src.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()

	if _, err := io.Copy(gz, src); err != nil {
		return fmt.Errorf("compress: %w", err)
	}
	return nil
}

func (h *BackupHandler) backupPostgres(dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()

	cmd := exec.Command("pg_dump", "--no-password", "--format=plain", h.dbURL)
	cmd.Stdout = gz
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func (h *BackupHandler) backupMySQL(dest string) error {
	user, pass, host, dbname, err := parseMySQLURL(h.dbURL)
	if err != nil {
		return fmt.Errorf("parse db url: %w", err)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()

	cmd := exec.Command("mysqldump", "-u", user, "-p"+pass, "-h", host, dbname)
	cmd.Stdout = gz
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// ── Restore implementations ───────────────────────────────────────────────────

func (h *BackupHandler) restoreSQLite(src string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}
	defer gz.Close()

	out, err := os.Create(h.sqlitePath + ".restoring")
	if err != nil {
		return fmt.Errorf("create staging file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, gz); err != nil {
		os.Remove(h.sqlitePath + ".restoring")
		return fmt.Errorf("write staging file: %w", err)
	}
	out.Close()

	return os.Rename(h.sqlitePath+".restoring", h.sqlitePath)
}

func (h *BackupHandler) restorePostgres(src string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}
	defer gz.Close()

	cmd := exec.Command("psql", h.dbURL)
	cmd.Stdin = gz
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func (h *BackupHandler) restoreMySQL(src string) error {
	user, pass, host, dbname, err := parseMySQLURL(h.dbURL)
	if err != nil {
		return fmt.Errorf("parse db url: %w", err)
	}

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}
	defer gz.Close()

	cmd := exec.Command("mysql", "-u", user, "-p"+pass, "-h", host, dbname)
	cmd.Stdin = gz
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func isValidBackupName(name string) bool {
	return strings.HasPrefix(name, "scutum-") &&
		!strings.Contains(name, "/") &&
		!strings.Contains(name, "..") &&
		(strings.HasSuffix(name, ".gz"))
}

func driverFromFilename(name string) string {
	switch {
	case strings.Contains(name, ".pgdump"):
		return "postgres"
	case strings.Contains(name, ".sql"):
		return "mysql"
	default:
		return "sqlite"
	}
}

func restoreMessage(driver string) string {
	if driver == "sqlite" {
		return "Database file replaced. Restart Scutum to complete the restore."
	}
	return "Database restored successfully."
}

// parseMySQLURL extracts user/pass/host/dbname from mysql://user:pass@host/db
func parseMySQLURL(u string) (user, pass, host, dbname string, err error) {
	u = strings.TrimPrefix(u, "mysql://")
	userPart, rest, ok := strings.Cut(u, "@")
	if !ok {
		return "", "", "", "", fmt.Errorf("invalid mysql url")
	}
	user, pass, _ = strings.Cut(userPart, ":")
	host, dbname, _ = strings.Cut(rest, "/")
	return
}
