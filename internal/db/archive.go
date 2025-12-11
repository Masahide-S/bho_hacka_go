package db

import (
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ArchiveOldData は指定期間より古いデータをCSV.gzに退避して削除します
func (s *Store) ArchiveOldData(retention time.Duration) error {
	// アーカイブフォルダ作成
	home, _ := os.UserHomeDir()
	archiveDir := filepath.Join(home, ".devmon", "archive")
	os.MkdirAll(archiveDir, 0755)

	threshold := time.Now().Add(-retention).Format("2006-01-02 15:04:05")

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. データ抽出
	rows, err := tx.Query("SELECT id, timestamp, cpu_usage FROM system_metrics WHERE timestamp < ?", threshold)
	if err != nil {
		return err
	}

	// データがなければ終了
	if !rows.Next() {
		rows.Close()
		return nil
	}

	// 2. CSV書き出し
	filename := fmt.Sprintf("metrics_%s.csv.gz", time.Now().Format("20060102_150405"))
	f, err := os.Create(filepath.Join(archiveDir, filename))
	if err != nil {
		rows.Close()
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	cw := csv.NewWriter(gw)
	cw.Write([]string{"id", "timestamp", "cpu_usage"}) // ヘッダー

	for {
		var id int64
		var ts string
		var cpu float64
		rows.Scan(&id, &ts, &cpu)
		cw.Write([]string{strconv.FormatInt(id, 10), ts, fmt.Sprintf("%.2f", cpu)})
		if !rows.Next() {
			break
		}
	}
	rows.Close()
	cw.Flush()
	gw.Close()

	// 3. データ削除
	_, err = tx.Exec("DELETE FROM system_metrics WHERE timestamp < ?", threshold)
	if err != nil {
		return err
	}

	return tx.Commit()
}
