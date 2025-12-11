package ui

import (
	"fmt"
)

// renderSystemResourcesDetail renders detailed system resources information
func (m Model) renderSystemResourcesDetail() string {
	sr := m.systemResources

	// CPU情報
	cpuSection := fmt.Sprintf(`CPU使用率:
  全体: %.1f%% (%dコア)`, sr.CPUUsage, sr.CPUCores)

	// メモリ情報（Activity Monitor形式）
	memorySection := fmt.Sprintf(`
メモリ使用状況: (Activity Monitor形式)
  使用中: %.2fGB / %.2fGB (%.1f%%)

  内訳:
    App Memory (Active):  %.2fGB
    Wired Memory:         %.2fGB
    Compressed:           %.2fGB
    Cached Files:         %.2fGB

  使用可能:               %.2fGB`,
		float64(sr.MemoryUsed)/1024.0,
		float64(sr.MemoryTotal)/1024.0,
		sr.MemoryPerc,
		float64(sr.MemoryAppMemory)/1024.0,
		float64(sr.MemoryWired)/1024.0,
		float64(sr.MemoryCompressed)/1024.0,
		float64(sr.MemoryCached)/1024.0,
		float64(sr.MemoryAvailable)/1024.0,
	)

	// ストレージ情報
	storageSection := fmt.Sprintf(`
ストレージ使用状況:
  使用中: %dGB / %dGB (%.1f%%)
  空き容量: %dGB`,
		sr.StorageUsed,
		sr.StorageTotal,
		sr.StoragePerc,
		sr.StorageFree,
	)

	// その他の情報
	otherSection := fmt.Sprintf(`
その他の情報:
  プロセス数: %d
  システム稼働時間: %s`,
		sr.ProcessCount,
		sr.Uptime,
	)

	return cpuSection + memorySection + storageSection + otherSection
}
