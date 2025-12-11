package db

import "database/sql"

// GetRecentMetrics は「直近30分」の詳細データを取得します (グラフモード用)
// limit: データ点数（例: 100点）
func (s *Store) GetRecentMetrics(limit int) ([]float64, error) {
	query := `
	SELECT cpu_usage FROM (
		SELECT cpu_usage, timestamp FROM system_metrics
		ORDER BY timestamp DESC LIMIT ?
	) ORDER BY timestamp ASC
	`
	return s.fetchFloats(query, limit)
}

// GetLongTermMetrics は「過去3日間」のデータを「1時間平均」で取得します (ヒストリーモード用)
func (s *Store) GetLongTermMetrics(days int) ([]float64, error) {
	// SQLiteの strftime で時間ごとのグルーピングを行います
	query := `
	SELECT AVG(cpu_usage) as avg_usage
	FROM system_metrics
	WHERE timestamp > datetime('now', '-' || ? || ' days')
	GROUP BY strftime('%Y-%m-%d %H', timestamp)
	ORDER BY timestamp ASC
	`
	return s.fetchFloats(query, days)
}

// 内部ヘルパー関数
func (s *Store) fetchFloats(query string, args ...interface{}) ([]float64, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []float64
	for rows.Next() {
		var val sql.NullFloat64 // NULL対策
		if err := rows.Scan(&val); err != nil {
			return nil, err
		}
		if val.Valid {
			data = append(data, val.Float64)
		} else {
			data = append(data, 0.0)
		}
	}
	return data, nil
}
