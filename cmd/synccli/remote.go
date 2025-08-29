package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"synccli/remote"

	"github.com/spf13/cobra"
)

var (
	remoteConfigName string
	remoteHost       string
	remotePort       int
	remoteUser       string
	remotePassword   string
	remoteKeyFile    string
	remoteBasePath   string
	syncDirection    string
	dryRun           bool
	force            bool
	verbose          bool
	progress         bool
	deleteEctra      bool
	compression      bool
	encryption       bool
	incremental      bool
	knownHostsFile   string
	strictHostCheck  bool
)

// 远程同步根命令
var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Remote synchronous management",
	Long:  `Manage remote synchronization configuration and perform remote file synchronization operations.`,
}

// 远程配置管理命令
var remoteConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Remote synchronous management",
	Long:  `Manage remote synchronization configurations, including adding, deleting, listing, and updating them.`,
}

// 添加远程配置命令
var remoteConfigAddCmd = &cobra.Command{
	Use:   "add []",
	Short: "Remote synchronous management",
	Long:  `Add a new remote synchronization configuration.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configName := args[0]
		cm, err := remote.NewConfigManager()
		if err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}

		config := &remote.RemoteConfig{
			Name: configName,
			SSH: &remote.SSHConfig{
				Host:     remoteHost,
				Port:     remotePort,
				Username: remoteUser,
				Password: remotePassword,
				KeyFile:  remoteKeyFile,
				Timeout:  30,
			},
			RemoteBase:  remoteBasePath,
			Compression: compression,
			Encryption:  encryption,
			Incremental: incremental,
			ExcludeList: []string{
				".git", ".DS_Store", "*.tmp", "*.log",
				"node_modules", "__pycache__", "target",
			},
		}

		if err := cm.ValidateConfig(config); err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}
		if err := cm.AddConfig(config); err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}

		fmt.Printf("Add over : %s\n", configName)
		return nil
	},
}

// 列出远程配置命令
var remoteConfigListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all remote configurations.",
	Long:  `List all saved remote synchronization configurations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cm, err := remote.NewConfigManager()
		if err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}

		configs := cm.ListConfigs()
		if len(configs) == 0 {
			fmt.Println("null config")
			return nil
		}

		fmt.Println("The list:")
		fmt.Println("================")
		for name, config := range configs {
			fmt.Printf("the name: %s\n", name)
			fmt.Printf("  the host: %s:%d\n", config.SSH.Host, config.SSH.Port)
			fmt.Printf("  username: %s\n", config.SSH.Username)
			fmt.Printf("  remote path: %s\n", config.RemoteBase)
			fmt.Printf("  zip: %v, crypto: %v, add: %v\n",
				config.Compression, config.Encryption, config.Incremental)
			fmt.Println("----------------")
		}
		return nil
	},
}

// 删除远程配置命令
var remoteConfigRemoveCmd = &cobra.Command{
	Use:   "remove []",
	Short: "Delete a remote configuration.",
	Long:  `Delete the specified remote synchronization configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configName := args[0]
		cm, err := remote.NewConfigManager()
		if err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}
		if err := cm.RemoveConfig(configName); err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}

		fmt.Printf("remove over: %s", configName)
		return nil
	},
}

// 远程同步命令
var remoteSyncCmd = &cobra.Command{
	Use:   "sync [] []",
	Short: "Execute remote synchronization.",
	Long:  `Execute remote file synchronization operations.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		localPath := args[0]
		var remotePath string
		if len(args) > 1 {
			remotePath = args[1]
		} else {
			remotePath = filepath.Base(localPath)
		}

		var config *remote.RemoteConfig
		var err error

		if remoteConfigName != "" {
			cm, err := remote.NewConfigManager()
			if err != nil {
				return fmt.Errorf("Errorf: %v", err)
			}

			config, err = cm.GetConfig(remoteConfigName)
			if err != nil {
				return fmt.Errorf("Errorf: %v", err)
			}
		} else {
			if remoteHost == "" || remoteUser == "" {
				return fmt.Errorf("no hostname or username")
			}
			config = &remote.RemoteConfig{
				Name: "Temporary configuration.",
				SSH: &remote.SSHConfig{
					Host:            remoteHost,
					Port:            remotePort,
					Username:        remoteUser,
					Password:        remotePassword,
					KeyFile:         remoteKeyFile,
					Timeout:         30,
					KnownHostsFile:  knownHostsFile,
					StrictHostCheck: strictHostCheck,
				},
				RemoteBase:  remoteBasePath,
				Compression: compression,
				Encryption:  encryption,
				Incremental: incremental,
				ExcludeList: []string{
					".git", ".DS_Store", "*.tmp", "*.log",
					"node_modules", "__pycache__", "target",
				},
			}
		}

		// 解释同步方向
		var direction remote.SyncDirection
		switch strings.ToLower(syncDirection) {
		case "upload", "to", "push":
			direction = remote.SyncToRemote
		case "download", "from", "pull":
			direction = remote.SyncFromRemote
		case "both", "bidirectional":
			direction = remote.SyncBoth
		default:
			direction = remote.SyncToRemote
		}

		options := &remote.SyncOptions{
			Direction:   direction,
			DryRun:      dryRun,
			Force:       force,
			Verbose:     verbose,
			Progress:    progress,
			DeleteExtra: deleteEctra,
		}

		engine := remote.NewRemoteSyncEngine(config, options)

		fmt.Printf("connect %s@%s:%d...\n", config.SSH.Username, config.SSH.Host, config.SSH.Port)

		if err := engine.Connect(); err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}
		defer engine.Disconnect()

		fmt.Printf("connect over")

		fmt.Printf("starting upload: %s <-> %s\n", localPath, remotePath)

		result, err := engine.SyncDirectory(localPath, remotePath)
		if err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}

		fmt.Println("\n=== Synchronization Complete ===")
		fmt.Printf("Total Files: %d\n", result.TotalFiles)
		fmt.Printf("Uploaded Files: %d\n", result.UploadedFiles)
		fmt.Printf("Downloaded Files: %d\n", result.DownloadFiles)
		fmt.Printf("Deleted Files: %d\n", result.DeletedFiles)
		fmt.Printf("Error Files: %d\n", result.ErrorFiles)
		fmt.Printf("Transferred Size: %s\n", formatBytes(result.TotalSize))
		fmt.Printf("Time Taken: %v\n", result.Duration)

		if len(result.Errors) > 0 {
			fmt.Println("\nError List:")
			for _, err := range result.Errors {
				fmt.Printf(" - %s\n", err)
			}
		}

		return nil
	},
}

// 测试远程连接命令
var remoteTestCmd = &cobra.Command{
	Use:   "test []",
	Short: "Test remote connection.",
	Long:  `Test if the remote connection for the specified configuration is working properly.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configName := args[0]
		cm, err := remote.NewConfigManager()
		if err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}

		config, err := cm.GetConfig(configName)
		if err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}

		fmt.Printf("test %s@%s:%d...\n", config.SSH.Username, config.SSH.Port)

		client := remote.NewSSHClient(config.SSH)
		if err := client.Connect(); err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}
		defer client.Close()

		output, err := client.ExecuteCommand("echo 'Hello from remote server'")
		if err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}
		fmt.Println("echo: %s", output)
		return nil
	},
}

// 格式化字节数
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func init() {
	// Add remote commands to root command
	rootCmd.AddCommand(remoteCmd)

	// Add subcommands
	remoteCmd.AddCommand(remoteConfigCmd)
	remoteCmd.AddCommand(remoteSyncCmd)
	remoteCmd.AddCommand(remoteTestCmd)

	// Add configuration management subcommands
	remoteConfigCmd.AddCommand(remoteConfigAddCmd)
	remoteConfigCmd.AddCommand(remoteConfigListCmd)
	remoteConfigCmd.AddCommand(remoteConfigRemoveCmd)

	// Configure add command parameters
	remoteConfigAddCmd.Flags().StringVar(&remoteHost, "host", "", "Remote host address (required)")
	remoteConfigAddCmd.Flags().IntVar(&remotePort, "port", 22, "SSH port")
	remoteConfigAddCmd.Flags().StringVar(&remoteUser, "user", "", "SSH username (required)")
	remoteConfigAddCmd.Flags().StringVar(&remotePassword, "password", "", "SSH password")
	remoteConfigAddCmd.Flags().StringVar(&remoteKeyFile, "key", "", "SSH private key file path")
	remoteConfigAddCmd.Flags().StringVar(&remoteBasePath, "base", "/tmp/synccli", "Remote base path")
	remoteConfigAddCmd.Flags().BoolVar(&compression, "compression", true, "Enable compression")
	remoteConfigAddCmd.Flags().BoolVar(&encryption, "encryption", true, "Enable encryption")
	remoteConfigAddCmd.Flags().BoolVar(&incremental, "incremental", true, "Enable incremental sync")

	remoteConfigAddCmd.MarkFlagRequired("host")
	remoteConfigAddCmd.MarkFlagRequired("user")

	// Remote sync command parameters
	remoteSyncCmd.Flags().StringVar(&remoteConfigName, "config", "", "Use saved configuration name")
	remoteSyncCmd.Flags().StringVar(&remoteHost, "host", "", "Remote host address")
	remoteSyncCmd.Flags().IntVar(&remotePort, "port", 22, "SSH port")
	remoteSyncCmd.Flags().StringVar(&remoteUser, "user", "", "SSH username")
	remoteSyncCmd.Flags().StringVar(&remotePassword, "password", "", "SSH password")
	remoteSyncCmd.Flags().StringVar(&remoteKeyFile, "key", "", "SSH private key file path")
	remoteSyncCmd.Flags().StringVar(&remoteBasePath, "base", "/tmp/synccli", "Remote base path")

	remoteSyncCmd.Flags().StringVar(&knownHostsFile, "known-hosts", "", "known_hosts file path (default: ~/.ssh/known_hosts)")
	remoteSyncCmd.Flags().BoolVar(&strictHostCheck, "strict-host-check", true, "Enable strict host key checking")
	remoteSyncCmd.Flags().StringVar(&syncDirection, "direction", "upload", "Sync direction (upload/download/both)")
	remoteSyncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run, no actual execution")
	remoteSyncCmd.Flags().BoolVar(&force, "force", false, "Force overwrite files")
	remoteSyncCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed information")
	remoteSyncCmd.Flags().BoolVar(&progress, "progress", true, "Show progress bar")
	remoteSyncCmd.Flags().BoolVar(&deleteEctra, "delete", false, "Delete extra files")
	remoteSyncCmd.Flags().BoolVar(&compression, "compression", true, "Enable compression")
	remoteSyncCmd.Flags().BoolVar(&encryption, "encryption", true, "Enable encryption")
	remoteSyncCmd.Flags().BoolVar(&incremental, "incremental", true, "Enable incremental sync")
}
