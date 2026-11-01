// Package store is the SQLite persistence layer.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	StatusQueued      = "queued"
	StatusDownloading = "downloading"
	StatusPaused      = "paused"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"

	SettingDownloadProxy = "download_proxy"
)

var ErrNotFound = errors.New("not found")

type User struct {
	ID        int64
	Username  string
	Token     string
	Role      string
	CreatedAt time.Time
	Downloads int64
}

type Download struct {
	ID              string
	UserID          int64
	Username        string
	FileToken       string
	URL             string
	VideoID         string
	Title           string
	Kind            string
	Quality         string
	Status          string
	Progress        float64
	BytesDownloaded int64
	TotalBytes      int64
	SpeedBPS        int64
	ETA             string
	FilePath        string
	FileName        string
	FileSize        int64
	Error           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
}

type ProgressUpdate struct {
	Percent         float64
	BytesDownloaded int64
	TotalBytes      int64
	SpeedBPS        int64
	ETA             string
}

type Stats struct {
	DownloadsTotal     int64 `json:"downloads_total"`
	DownloadsCompleted int64 `json:"downloads_completed"`
	BytesDownloaded    int64 `json:"bytes_downloaded"`
	Active             int64 `json:"active"`
	Queued             int64 `json:"queued"`
	Paused             int64 `json:"paused"`
	Failed             int64 `json:"failed"`
	Users              int64 `json:"users,omitempty"`
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	if err := applyPragmas(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func sqliteDSN(path string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	u.RawQuery = "_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	return u.String()
}

func applyPragmas(db *sql.DB) error {
	for _, q := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("%s: %w", q, err)
		}
	}
	return nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (1)`)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL,
  token TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
  created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS downloads (
  id TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id),
  file_token TEXT NOT NULL UNIQUE,
  url TEXT NOT NULL,
  video_id TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL CHECK (kind IN ('video', 'audio')),
  quality TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('queued', 'downloading', 'paused', 'completed', 'failed', 'cancelled')),
  progress REAL NOT NULL DEFAULT 0,
  bytes_downloaded INTEGER NOT NULL DEFAULT 0,
  total_bytes INTEGER NOT NULL DEFAULT 0,
  speed_bps INTEGER NOT NULL DEFAULT 0,
  eta TEXT NOT NULL DEFAULT '',
  file_path TEXT NOT NULL DEFAULT '',
  file_name TEXT NOT NULL DEFAULT '',
  file_size INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_downloads_user_created ON downloads(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_downloads_status_created ON downloads(status, created_at);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
`

func (s *Store) EnsureAdmin(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("admin key required")
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE role = 'admin' ORDER BY id LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.db.ExecContext(ctx, `INSERT INTO users (username, token, role, created_at) VALUES ('admin', ?, 'admin', ?)`, token, now())
		return err
	}
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE users SET token = ? WHERE id = ?`, token, id)
	return err
}

func (s *Store) CreateUser(ctx context.Context, username, token string) (*User, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO users (username, token, role, created_at) VALUES (?, ?, 'user', ?)`, username, token, now())
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.UserByID(ctx, id)
}

func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, username, token, role, created_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) UserByToken(ctx context.Context, token string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, username, token, role, created_at FROM users WHERE token = ?`, token)
	return scanUser(row)
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.role, u.created_at, COUNT(d.id)
		FROM users u
		LEFT JOIN downloads d ON d.user_id = u.id
		GROUP BY u.id
		ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var created string
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &created, &u.Downloads); err != nil {
			return nil, err
		}
		u.CreatedAt = parseTime(created)
		out = append(out, u)
	}
	if out == nil {
		out = []User{}
	}
	return out, rows.Err()
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ? AND role <> 'admin'`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateDownload(ctx context.Context, d *Download) error {
	ts := now()
	var lastErr error
	for i := 0; i < 3; i++ {
		id, err := randHex(16)
		if err != nil {
			return err
		}
		token, err := randHex(32)
		if err != nil {
			return err
		}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO downloads (
				id, user_id, file_token, url, video_id, title, kind, quality, status,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, d.UserID, token, d.URL, d.VideoID, d.Title, d.Kind, d.Quality, StatusQueued, ts, ts)
		if err == nil {
			d.ID = id
			d.FileToken = token
			d.Status = StatusQueued
			d.CreatedAt = parseTime(ts)
			d.UpdatedAt = d.CreatedAt
			return nil
		}
		lastErr = err
		if !isUnique(err) {
			return err
		}
	}
	return fmt.Errorf("allocate download id: %w", lastErr)
}

func (s *Store) GetDownload(ctx context.Context, id string) (*Download, error) {
	row := s.db.QueryRowContext(ctx, downloadSelect+` WHERE d.id = ?`, id)
	return scanDownload(row)
}

func (s *Store) GetByFileToken(ctx context.Context, token string) (*Download, error) {
	row := s.db.QueryRowContext(ctx, downloadSelect+` WHERE d.file_token = ?`, token)
	return scanDownload(row)
}

func (s *Store) FindActive(ctx context.Context, userID int64, page, kind, quality string) (*Download, error) {
	row := s.db.QueryRowContext(ctx, downloadSelect+`
		WHERE d.user_id = ? AND d.url = ? AND d.kind = ? AND d.quality = ?
		  AND d.status IN ('queued', 'downloading', 'paused')
		ORDER BY d.created_at DESC LIMIT 1`, userID, page, kind, quality)
	d, err := scanDownload(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return d, err
}

func (s *Store) ListDownloads(ctx context.Context, userID int64, all bool, limit int) ([]Download, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	q := downloadSelect
	args := []any{}
	if !all {
		q += ` WHERE d.user_id = ?`
		args = append(args, userID)
	}
	q += ` ORDER BY d.created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Download
	for rows.Next() {
		d, err := scanDownload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	if out == nil {
		out = []Download{}
	}
	return out, rows.Err()
}

// ListAllByUser returns every download for deletion. Capped at 10000.
func (s *Store) ListAllByUser(ctx context.Context, userID int64) ([]Download, error) {
	rows, err := s.db.QueryContext(ctx, downloadSelect+` WHERE d.user_id = ? ORDER BY d.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Download
	for rows.Next() {
		d, err := scanDownload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
		if len(out) >= 10000 {
			break
		}
	}
	return out, rows.Err()
}

func (s *Store) ClaimNext(ctx context.Context) (*Download, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM downloads
		WHERE status = 'queued'
		ORDER BY created_at ASC
		LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ts := now()
	res, err := tx.ExecContext(ctx, `
		UPDATE downloads
		SET status = 'downloading', updated_at = ?, started_at = COALESCE(started_at, ?), error = ''
		WHERE id = ? AND status = 'queued'`, ts, ts, id)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetDownload(ctx, id)
}

func (s *Store) RecoverInterrupted(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE downloads SET status = 'queued', updated_at = ?, speed_bps = 0
		WHERE status = 'downloading'`, now())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) UpdateProgress(ctx context.Context, id string, p ProgressUpdate) error {
	if p.Percent < 0 {
		p.Percent = 0
	}
	if p.Percent > 100 {
		p.Percent = 100
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE downloads
		SET progress = ?, bytes_downloaded = ?, total_bytes = CASE WHEN ? > 0 THEN ? ELSE total_bytes END,
		    speed_bps = ?, eta = ?, updated_at = ?
		WHERE id = ? AND status = 'downloading'`,
		p.Percent, p.BytesDownloaded, p.TotalBytes, p.TotalBytes, p.SpeedBPS, p.ETA, now(), id)
	return err
}

func (s *Store) UpdateMeta(ctx context.Context, id, title, videoID string) error {
	title = strings.TrimSpace(title)
	videoID = strings.TrimSpace(videoID)
	if title == "" && videoID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE downloads
		SET title = CASE WHEN ? <> '' THEN ? ELSE title END,
		    video_id = CASE WHEN ? <> '' THEN ? ELSE video_id END,
		    updated_at = ?
		WHERE id = ? AND status IN ('queued', 'downloading', 'paused')`,
		title, title, videoID, videoID, now(), id)
	return err
}

func (s *Store) SetStatus(ctx context.Context, id, status, errMsg string, from []string) error {
	if len(from) == 0 {
		return errors.New("missing status precondition")
	}
	placeholders := make([]string, len(from))
	args := []any{status, errMsg, now(), id}
	for i, st := range from {
		placeholders[i] = "?"
		args = append(args, st)
	}
	q := `UPDATE downloads SET status = ?, error = ?, updated_at = ?, speed_bps = 0 WHERE id = ? AND status IN (` + strings.Join(placeholders, ",") + `)`
	if status == StatusFailed || status == StatusCancelled || status == StatusCompleted {
		q = `UPDATE downloads SET status = ?, error = ?, updated_at = ?, speed_bps = 0, finished_at = ? WHERE id = ? AND status IN (` + strings.Join(placeholders, ",") + `)`
		args = []any{status, errMsg, now(), now(), id}
		for _, st := range from {
			args = append(args, st)
		}
	}
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkCompleted(ctx context.Context, id, path, name string, size int64) error {
	ts := now()
	res, err := s.db.ExecContext(ctx, `
		UPDATE downloads
		SET status = 'completed', progress = 100, file_path = ?, file_name = ?, file_size = ?,
		    error = '', speed_bps = 0, eta = '', bytes_downloaded = ?, total_bytes = CASE WHEN total_bytes > 0 THEN total_bytes ELSE ? END,
		    updated_at = ?, finished_at = ?
		WHERE id = ? AND status = 'downloading'`,
		path, name, size, size, size, ts, ts, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteDownload(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM downloads WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Stats(ctx context.Context, userID int64, all bool) (Stats, error) {
	q := `
		SELECT
		  COUNT(*),
		  COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN status = 'completed' THEN file_size ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN status = 'downloading' THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN status = 'paused' THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0)
		FROM downloads`
	args := []any{}
	if !all {
		q += ` WHERE user_id = ?`
		args = append(args, userID)
	}
	var st Stats
	if err := s.db.QueryRowContext(ctx, q, args...).Scan(
		&st.DownloadsTotal, &st.DownloadsCompleted, &st.BytesDownloaded,
		&st.Active, &st.Queued, &st.Paused, &st.Failed,
	); err != nil {
		return Stats{}, err
	}
	if all {
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'user'`).Scan(&st.Users); err != nil {
			return Stats{}, err
		}
	}
	return st, nil
}

const downloadSelect = `
SELECT d.id, d.user_id, u.username, d.file_token, d.url, d.video_id, d.title, d.kind, d.quality,
       d.status, d.progress, d.bytes_downloaded, d.total_bytes, d.speed_bps, d.eta,
       d.file_path, d.file_name, d.file_size, d.error, d.created_at, d.updated_at, d.started_at, d.finished_at
FROM downloads d
JOIN users u ON u.id = d.user_id`

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (*User, error) {
	var u User
	var created string
	err := row.Scan(&u.ID, &u.Username, &u.Token, &u.Role, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.CreatedAt = parseTime(created)
	return &u, nil
}

func scanDownload(row scanner) (*Download, error) {
	var d Download
	var created, updated string
	var started, finished sql.NullString
	err := row.Scan(
		&d.ID, &d.UserID, &d.Username, &d.FileToken, &d.URL, &d.VideoID, &d.Title, &d.Kind, &d.Quality,
		&d.Status, &d.Progress, &d.BytesDownloaded, &d.TotalBytes, &d.SpeedBPS, &d.ETA,
		&d.FilePath, &d.FileName, &d.FileSize, &d.Error, &created, &updated, &started, &finished,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.CreatedAt = parseTime(created)
	d.UpdatedAt = parseTime(updated)
	if started.Valid {
		t := parseTime(started.String)
		d.StartedAt = &t
	}
	if finished.Valid {
		t := parseTime(finished.String)
		d.FinishedAt = &t
	}
	return &d, nil
}

// GetSetting returns the stored value. ok is false when the key has never been set.
func (s *Store) GetSetting(ctx context.Context, key string) (value string, ok bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// SetSetting inserts or replaces a system setting.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now())
	return err
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "UNIQUE")
}

// NewToken is a user API key.
func NewToken() (string, error) { return randHex(32) }
