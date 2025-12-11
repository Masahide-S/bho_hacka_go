package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/Masahide-S/bho_hacka_go/internal/db"
	"github.com/Masahide-S/bho_hacka_go/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:   "devmon",
	Short: "Local development environment monitor",
	Long:  `devmon monitors your local development services like PostgreSQL, Docker, Node.js, etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		// 1. データベースの初期化
		store, err := db.NewStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()

		// 2. バックグラウンドでアーカイブタスクを実行 (Goroutine)
		go func() {
			// 起動時にまず古いデータを整理
			store.ArchiveOldData(72 * time.Hour)

			// 以降、1時間ごとにチェック
			ticker := time.NewTicker(1 * time.Hour)
			for range ticker.C {
				store.ArchiveOldData(72 * time.Hour)
			}
		}()

		// 3. StoreをUIモデルに渡してTUIモードで起動
		if err := ui.RunWithStore(store); err != nil {
			fmt.Printf("エラー: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}