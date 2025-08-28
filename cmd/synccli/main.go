package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version    = "1.0.0"
	buildDate  = "2025"
	commitHash = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "synccli",
	Short: "FileSync CLI - Multi-language File Synchronization Tool",
	Long: `FileSync CLI is a high-performance, multi-language file synchronization command-line tool.
It supports incremental synchronization between local and remote directories, file encryption, compression, and user-defined rules.
By combining the strengths of Go, Python, Lua, and Rust, it delivers a highly concurrent, high-performance, and extensible file synchronization service.**
`,
	Version: version,
}

func init() {
	// 子命令
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(versionCmd)

	// 版本模板
	rootCmd.SetVersionTemplate(`{{printf "FileSync CLI %s" .Version}}
		Build Date : {{printf "%s" "` + buildDate + `"}}
		Commit Hash : {{printf "%s" "` + commitHash + `"}}
		`)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show Version Information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("FileSync CLI %s\n", version)
		fmt.Printf("Build Date : %s\n", buildDate)
		fmt.Printf("Commit Hash : %s\n", commitHash)
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
