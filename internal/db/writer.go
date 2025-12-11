package db

import (
	"context"
	"time"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// SaveSnapshot はシステムメトリクスとプロセスリストを一括で保存します
func (s *Store) SaveSnapshot(sys monitor.SystemResources, procs []monitor.ProcessInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // エラー時はロールバック

	// 1. システムメトリクスの保存
	res, err := tx.ExecContext(ctx, `
		INSERT INTO system_metrics (timestamp, cpu_usage, memory_used, memory_total, disk_usage)
		VALUES (?, ?, ?, ?, ?)`,
		time.Now().UTC(),
		sys.CPUUsage,
		sys.MemoryUsed,
		sys.MemoryTotal,
		sys.DiskPerc,
	)
	if err != nil {
		return err
	}

	// 挿入した行のIDを取得
	metricID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	// 2. プロセススナップショットの保存（データがある場合のみ）
	if len(procs) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO process_snapshots (metric_id, process_name, pid, cpu_usage, memory_usage, is_dev_tool)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, p := range procs {
			_, err = stmt.ExecContext(ctx, metricID, p.Name, p.PID, p.CPU, p.Memory, p.IsDevTool)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
