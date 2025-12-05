package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

var rootCmd = &cobra.Command{
	Use:   "devmon",
	Short: "Local development environment monitor",
	Long:  `devmon monitors your local development services like PostgreSQL, Docker, Node.js, etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Local Development Monitor ===")
		fmt.Println()
		fmt.Println("監視機能を実装中...")
		// PostgreSQL監視を追加
    fmt.Println(monitor.CheckPostgres())
    // Docker監視を追加
    fmt.Println(monitor.CheckDocker())
    // Node.js監視を追加
    fmt.Println(monitor.CheckNodejs())
    // Python(flask,jupyter..)監視を追加
    fmt.Println(monitor.CheckPython())
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
