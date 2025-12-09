package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/Masahide-S/bho_hacka_go/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:   "devmon",
	Short: "Local development environment monitor",
	Long:  `devmon monitors your local development services like PostgreSQL, Docker, Node.js, etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		// TUIモードで起動
		if err := ui.Run(); err != nil {
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