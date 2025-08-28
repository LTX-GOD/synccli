package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
)

// 同步选项
type SyncOptions struct {
	SourcePath string
	DestPath   string
	RuleFile   string
	Encrypt    bool
	Compress   bool
	Verbose    bool
}

// 文件元数据
type FileMetadata struct {
	Path          string    `json:"path"`
	Hash          string    `json:"hash"`
	Size          string    `json:"size"`
	ModifiledTime time.Time `json:"modified_time"`
	Permissions   string    `json:"permissions"`
}

// python扫描
type ScanResult struct {
	SourceFiles []FileMetadata `json:"source_files"`
	DestFiles   []FileMetadata `json:"dest_files"`
	Status      bool           `json:"status"`
	Message     string         `json:"message,omitempty"`
}

// lua过滤结果
type FilterResult struct {
	FilteredFiles []FileMetadata `json:"filtered_files"`
	Status        bool           `json:"status"`
	Message       string         `json:"message,omitempty"`
}

var syncOptions SyncOptions

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Perform file synchronization operations",
	Long: `The sync command is used to perform file synchronization operations, supporting the following features:

- Incremental sync: Only synchronizes changed files
- Custom rules: Define synchronization rules through Lua scripts
- Encrypted transfer: Protect files with AES encryption
- Compressed transfer: Reduce transfer size with zlib compression
- Progress tracking: Real-time synchronization progress display`,
	Example: `  # Basic directory synchronization
  synccli sync --source /path/to/src --dest /path/to/dest

  # 单个文件同步
  synccli sync --source /path/to/file.txt --dest /path/to/dest

  # 使用规则文件
  synccli sync -s ./src -d ./dst -r lua/rules.lua

  # 启用加密和压缩
  synccli sync -s ./src -d ./dst --encrypt --compress`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().StringVarP(&syncOptions.SourcePath, "source", "s", "", "Source path (file or directory, required)")
	syncCmd.Flags().StringVarP(&syncOptions.DestPath, "dest", "d", "", "Destination directory path (required)")
	syncCmd.Flags().StringVarP(&syncOptions.RuleFile, "rule", "r", "", "Lua rule script path (optional)")
	syncCmd.Flags().BoolVarP(&syncOptions.Encrypt, "encrypt", "e", false, "Enable AES encryption")     // Changed to BoolVarP and added short flag
	syncCmd.Flags().BoolVarP(&syncOptions.Compress, "compress", "c", false, "Enable zlib compression") // Changed to BoolVarP and added short flag
	syncCmd.Flags().BoolVarP(&syncOptions.Verbose, "verbose", "v", false, "Verbose output")

	// 设置必填参数
	syncCmd.MarkFlagRequired("source")
	syncCmd.MarkFlagRequired("dest")
}

// 同步操作
func runSync(cmd *cobra.Command, args []string) error {
	if err := validataSyncOptions(); err != nil {
		return fmt.Errorf("Parameter validation failed: %v", err)
	}

	fmt.Println("Initiating file synchronization.")

	start := time.Now()

	// 1.python扫描整个目录
	fmt.Println("Scanning directory.")

	scanResult, err := scanDirectories()

	if err != nil {
		return fmt.Errorf("Error with %v", err)
	}

	fmt.Printf("Scanning completed - Source files: %d, Destination files: %d", len(scanResult.SourceFiles), len(scanResult.DestFiles))

	// 2.Lua规则过滤
	filesToSync := scanResult.SourceFiles
	if syncOptions.RuleFile != "" {
		fmt.Printf("Using lua rules: %s\n", syncOptions.RuleFile)
		FilterResult, err := filterFiles(scanResult.SourceFiles)
		if err != nil {
			return fmt.Errorf("rules is err: %v\n", err)
		}
		filesToSync = FilterResult.FilteredFiles
		fmt.Println("Filtering completed - Files to synchronize: %d\n", len(filesToSync))
	}

	// 3.Rust差异计算
	fmt.Println("Calculating file differences...")
	diffFiles, err := calculateDifferences(filesToSync, scanResult.DestFiles)
	if err != nil {
		return fmt.Errorf("failed to calculate differences: %v", err)
	}

	if len(diffFiles) == 0 {
		fmt.Println("All files are up to date, no synchronization needed")
		return nil
	}

	fmt.Printf("📋 Files to synchronize: %d\n", len(diffFiles))
	if syncOptions.Verbose {
		for _, file := range diffFiles {
			fmt.Printf("  - %s\n", file.Path)
		}
	}

	// 4.文件传输
	fmt.Println("Starting file transfer...")
	if err := transferFiles(diffFiles); err != nil {
		return fmt.Errorf("file transfer failed: %v", err)
	}

	// 完成
	duration := time.Since(start)
	fmt.Printf("Synchronization completed! Time taken: %v\n", duration.Round(time.Millisecond))
	fmt.Printf("Statistics: Transferred %d files\n", len(diffFiles))

	return nil

}

func validataSyncOptions() error {
	//检查路径
	if _, err := os.Stat(syncOptions.DestPath); os.IsNotExist(err) {
		return fmt.Errorf("The path is nothing: %s", syncOptions.SourcePath)
	}

	sourceInfo, err := os.Stat(syncOptions.SourcePath)
	if err != nil {
		return fmt.Errorf("Can't get the path: %v", err)
	}

	if _, err := os.Stat(syncOptions.SourcePath); os.IsNotExist(err) {
		if !sourceInfo.IsDir() {
			if err := os.MkdirAll(syncOptions.DestPath, 0755); err != nil {
				return fmt.Errorf("Can't mkdir: %v", err)
			} else {
				if err := os.MkdirAll(syncOptions.DestPath, 0755); err != nil {
					return fmt.Errorf("Can't mkdir: %v", err)
				}
			}
		}
	}

	// 检查lua文件
	if syncOptions.RuleFile != "" {
		if _, err := os.Stat(syncOptions.RuleFile); os.IsNotExist(err) {
			return fmt.Errorf("The lua file is nothing: %s", syncOptions.RuleFile)
		}
	}
	return nil
}

func scanDirectories() (*ScanResult, error) {
	pythonScript := filepath.Join("python", "scanner.py")

	args := []string{pythonScript, syncOptions.SourcePath, syncOptions.DestPath}

	cmd := exec.Command("python3", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Python scan filed: %v", err)
	}

	// 解析json
	var result ScanResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("Scan result is nothing: %v", err)
	}
	if !result.Status {
		return nil, fmt.Errorf("Scan failed: %s", result.Message)
	}
	return &result, nil
}

func filterFiles(files []FileMetadata) (*FilterResult, error) {
	// 文件列表转换成json
	fileJSON, err := json.Marshal(files)
	if err != nil {
		return nil, fmt.Errorf("filed with: %v", err)
	}

	// 构建Lua
	cmd := exec.Command("lua", syncOptions.RuleFile, string(fileJSON))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Lua filed: %v", err)
	}

	// 解析结果
	var result FilterResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("filed with: %v", err)
	}

	if !result.Status {
		return nil, fmt.Errorf("filed: %s", result.Message)
	}
	return &result, nil
}

// 调用rust计算文件差异
func calculateDifferences(sourceFiles, destFiles []FileMetadata) ([]FileMetadata, error) {
	var diffFiles []FileMetadata

	// 创建目标文件映射
	destMap := make(map[string]FileMetadata)
	for _, file := range sourceFiles {
		destMap[file.Path] = file
	}

	// 比较文件
	for _, srcFile := range sourceFiles {
		relPath, _ := filepath.Rel(syncOptions.SourcePath, srcFile.Path)
		destPath := filepath.Join(syncOptions.DestPath, relPath)

		if destFile, exists := destMap[destPath]; !exists {
			diffFiles = append(diffFiles, srcFile)
		} else if srcFile.Hash != destFile.Hash {
			diffFiles = append(diffFiles, srcFile)
		}
	}
	return diffFiles, nil
}

// 传输文件
func transferFiles(files []FileMetadata) error {
	// 进度条
	bar := pb.StartNew(len(files))
	bar.SetTemplate(pb.Simple)

	// 检查是否为文件
	sourceInfo, err := os.Stat(syncOptions.SourcePath)
	if err != nil {
		return fmt.Errorf("Can't get the path: %v", err)
	}

	for _, file := range files {
		var destPath string

		if !sourceInfo.IsDir() {
			fileName := filepath.Base(file.Path)
			destPath = filepath.Join(syncOptions.DestPath, fileName)
		} else {
			relPath, err := filepath.Rel(syncOptions.SourcePath, file.Path)
			if err != nil {
				return fmt.Errorf("Error with: %v", err)
			}
			destPath = filepath.Join(syncOptions.DestPath, relPath)
		}

		// 创建目标目录
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("filed with mkdir: %v", err)
		}

		// 复制文件
		if err := copyFile(file.Path, destPath); err != nil {
			return fmt.Errorf("Copy filed %s: %v", file.Path, err)
		}
		bar.Increment()
	}
	bar.Finish()
	return nil
}

// 复制文件
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}
