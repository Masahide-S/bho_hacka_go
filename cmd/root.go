package cmd

import (
	"fmt"
	"os"

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

		// 2. StoreをUIモデルに渡してTUIモードで起動
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