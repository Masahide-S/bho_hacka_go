package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

type Store struct {
	db *sql.DB
}

// NewStore はDB接続を初期化し、テーブル作成と設定を行います
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dbDir := filepath.Join(home, ".devmon")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dbDir, "metrics.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// 【重要】WALモード有効化（UIとWorkerの同時アクセス時のロック競合を防ぐ）
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, err
	}
	// 同期設定をNORMALに（安全性と速度のバランス）
	if _, err := db.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	// 起動時に3日より前のデータを削除（容量圧迫防止）
	go store.cleanupOldData(3 * 24 * time.Hour)

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	// 親テーブル：システム全体のメトリクス
	// 子テーブル：その時点でのプロセススナップショット
	query := `
	CREATE TABLE IF NOT EXISTS system_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		cpu_usage REAL,
		memory_used INTEGER,
		memory_total INTEGER,
		disk_usage REAL
	);

	CREATE TABLE IF NOT EXISTS process_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		metric_id INTEGER,
		process_name TEXT,
		pid TEXT,
		cpu_usage REAL,
		memory_usage INTEGER,
		is_dev_tool BOOLEAN,
		FOREIGN KEY(metric_id) REFERENCES system_metrics(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON system_metrics(timestamp);
	CREATE INDEX IF NOT EXISTS idx_snapshots_metric_id ON process_snapshots(metric_id);
	`
	_, err := s.db.Exec(query)
	return err
}

func (s *Store) cleanupOldData(retention time.Duration) {
	// カスケード削除が効くように設定
	s.db.Exec("PRAGMA foreign_keys = ON;")
	hours := fmt.Sprintf("-%d hours", int(retention.Hours()))
	s.db.Exec("DELETE FROM system_metrics WHERE timestamp < datetime('now', ?)", hours)
}

// SaveMetric は現在のメトリクスを保存します（シンプル版）
func (s *Store) SaveMetric(cpu, disk float64, memUsed, memTotal int64) error {
	_, err := s.db.Exec(`
		INSERT INTO system_metrics (timestamp, cpu_usage, memory_used, memory_total, disk_usage)
		VALUES (?, ?, ?, ?, ?)`,
		time.Now().UTC(), cpu, memUsed, memTotal, disk,
	)
	return err
}
