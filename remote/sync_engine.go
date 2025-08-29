package remote

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"synccli/bindings"

	"github.com/cheggaaa/pb/v3"
)

// SyncDirection 同步方向
type SyncDirection int

const (
	SyncToRemote   SyncDirection = iota // 本地到远程
	SyncFromRemote                      // 远程到本地
	SyncBoth                            // 双向同步
)

// SyncOptions 同步选项
type SyncOptions struct {
	Direction   SyncDirection `json:"direction"`   // 同步方向
	DryRun      bool          `json:"dryRun"`      // 是否为试运行
	Force       bool          `json:"force"`       // 是否强制覆盖
	Verbose     bool          `json:"verbose"`     // 是否显示详细信息
	Progress    bool          `json:"progress"`    // 是否显示进度条
	DeleteExtra bool          `json:"deleteExtra"` // 是否删除多余文件
}

// SyncResult 同步结果
type SyncResult struct {
	TotalFiles    int           `json:"totalFiles"`    // 总文件数
	UploadedFiles int           `json:"uploadedFiles"` // 上传文件数
	DownloadFiles int           `json:"downloadFiles"` // 下载文件数
	DeletedFiles  int           `json:"deletedFiles"`  // 删除文件数
	SkippedFiles  int           `json:"skippedFiles"`  // 跳过文件数
	ErrorFiles    int           `json:"errorFiles"`    // 错误文件数
	TotalSize     int64         `json:"totalSize"`     // 总大小
	Duration      time.Duration `json:"duration"`      // 耗时
	Errors        []string      `json:"errors"`        // 错误列表
}

// RemoteSyncEngine 远程同步引擎
type RemoteSyncEngine struct {
	config       *RemoteConfig
	sshClient    *SSHClient
	options      *SyncOptions
	pythonClient *bindings.PythonInterface
}

// NewRemoteSyncEngine 创建新的远程同步引擎
func NewRemoteSyncEngine(config *RemoteConfig, options *SyncOptions) *RemoteSyncEngine {
	return &RemoteSyncEngine{
		config:       config,
		options:      options,
		pythonClient: bindings.NewPythonInterface(options.Verbose),
	}
}

// Connect 连接到远程服务器
func (rse *RemoteSyncEngine) Connect() error {
	rse.sshClient = NewSSHClient(rse.config.SSH)
	return rse.sshClient.Connect()
}

// Disconnect 断开远程连接
func (rse *RemoteSyncEngine) Disconnect() error {
	if rse.sshClient != nil {
		return rse.sshClient.Close()
	}
	return nil
}

// SyncDirectory 同步目录
func (rse *RemoteSyncEngine) SyncDirectory(localPath, remotePath string) (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{
		Errors: make([]string, 0),
	}

	if !rse.sshClient.IsConnected() {
		return nil, fmt.Errorf("SSH未连接")
	}

	// 如果是相对路径，转换为绝对路径
	if !filepath.IsAbs(remotePath) {
		remotePath = filepath.Join(rse.config.RemoteBase, remotePath)
	}

	// 确保远程目录存在
	if err := rse.ensureRemoteDirectory(remotePath); err != nil {
		return nil, fmt.Errorf("创建远程目录失败: %v", err)
	}

	// 扫描本地文件
	localFiles, err := rse.scanLocalFiles(localPath)
	if err != nil {
		return nil, fmt.Errorf("扫描本地文件失败: %v", err)
	}

	// 扫描远程文件
	remoteFiles, err := rse.scanRemoteFiles(remotePath)
	if err != nil {
		return nil, fmt.Errorf("扫描远程文件失败: %v", err)
	}

	// 计算同步计划
	syncPlan := rse.calculateSyncPlan(localFiles, remoteFiles, localPath, remotePath)
	result.TotalFiles = len(syncPlan.Upload) + len(syncPlan.Download) + len(syncPlan.Delete)

	if rse.options.Verbose {
		fmt.Printf("同步计划: 上传 %d, 下载 %d, 删除 %d\n",
			len(syncPlan.Upload), len(syncPlan.Download), len(syncPlan.Delete))
	}

	if rse.options.DryRun {
		rse.printSyncPlan(syncPlan)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// 执行同步
	if err := rse.executeSyncPlan(syncPlan, result); err != nil {
		return result, err
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// FileInfo 文件信息
type FileInfo struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	IsDir   bool      `json:"isDir"`
	Hash    string    `json:"hash,omitempty"`
}

// SyncPlan 同步计划
type SyncPlan struct {
	Upload   []SyncItem `json:"upload"`   // 需要上传的文件
	Download []SyncItem `json:"download"` // 需要下载的文件
	Delete   []SyncItem `json:"delete"`   // 需要删除的文件
}

// SyncItem 同步项目
type SyncItem struct {
	LocalPath  string `json:"localPath"`
	RemotePath string `json:"remotePath"`
	Size       int64  `json:"size"`
	Action     string `json:"action"` // upload, download, delete
}

// scanLocalFiles 扫描本地文件（使用Python扫描器）
func (rse *RemoteSyncEngine) scanLocalFiles(localPath string) (map[string]*FileInfo, error) {
	files := make(map[string]*FileInfo)

	// 使用Python扫描器扫描本地文件
	scanResult, err := rse.pythonClient.ScanDirectories(localPath, localPath)
	if err != nil {
		return nil, fmt.Errorf("Python扫描失败: %v", err)
	}

	// 转换Python扫描结果为FileInfo格式
	for _, pyFile := range scanResult.SourceFiles {
		// 计算相对路径
		relPath, err := filepath.Rel(localPath, pyFile.Path)
		if err != nil {
			continue
		}

		// 检查是否应该排除
		if rse.shouldExclude(relPath) {
			continue
		}

		// 解析时间字符串
		modTime, err := time.Parse("2006-01-02T15:04:05Z", pyFile.ModifiedTime)
		if err != nil {
			modTime = time.Now() // 如果解析失败，使用当前时间
		}

		fileInfo := &FileInfo{
			Path:    relPath,
			Size:    pyFile.Size,
			ModTime: modTime,
			IsDir:   false,       // Python扫描器只返回文件，不返回目录
			Hash:    pyFile.Hash, // 使用Python计算的哈希值
		}

		files[relPath] = fileInfo
	}

	return files, nil
}

// scanRemoteFiles 扫描远程文件
func (rse *RemoteSyncEngine) scanRemoteFiles(remotePath string) (map[string]*FileInfo, error) {
	files := make(map[string]*FileInfo)

	// 使用SSH命令递归列出文件
	command := fmt.Sprintf("find '%s' -type f -printf '%%P\\t%%s\\t%%T@\\n' 2>/dev/null || true", remotePath)
	output, err := rse.sshClient.ExecuteCommand(command)
	if err != nil {
		// 如果目录不存在，返回空列表
		return files, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}

		relPath := parts[0]
		if rse.shouldExclude(relPath) {
			continue
		}

		var size int64
		var modTime time.Time

		fmt.Sscanf(parts[1], "%d", &size)
		var timestamp float64
		fmt.Sscanf(parts[2], "%f", &timestamp)
		modTime = time.Unix(int64(timestamp), 0)

		files[relPath] = &FileInfo{
			Path:    relPath,
			Size:    size,
			ModTime: modTime,
			IsDir:   false,
		}
	}

	return files, nil
}

// shouldExclude 检查文件是否应该排除
func (rse *RemoteSyncEngine) shouldExclude(path string) bool {
	for _, pattern := range rse.config.ExcludeList {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// calculateSyncPlan 计算同步计划
func (rse *RemoteSyncEngine) calculateSyncPlan(localFiles, remoteFiles map[string]*FileInfo, localBase, remoteBase string) *SyncPlan {
	plan := &SyncPlan{
		Upload:   make([]SyncItem, 0),
		Download: make([]SyncItem, 0),
		Delete:   make([]SyncItem, 0),
	}

	// 检查需要上传或更新的文件
	if rse.options.Direction == SyncToRemote || rse.options.Direction == SyncBoth {
		for relPath, localFile := range localFiles {
			if localFile.IsDir {
				continue
			}

			localFullPath := filepath.Join(localBase, relPath)
			remoteFullPath := filepath.Join(remoteBase, relPath)

			remoteFile, exists := remoteFiles[relPath]
			shouldUpload := false

			if !exists {
				shouldUpload = true
			} else if rse.config.Incremental {
				// 增量同步：比较修改时间和大小
				if localFile.ModTime.After(remoteFile.ModTime) || localFile.Size != remoteFile.Size {
					shouldUpload = true
				}
			} else {
				shouldUpload = rse.options.Force
			}

			if shouldUpload {
				plan.Upload = append(plan.Upload, SyncItem{
					LocalPath:  localFullPath,
					RemotePath: remoteFullPath,
					Size:       localFile.Size,
					Action:     "upload",
				})
			}
		}
	}

	// 检查需要下载的文件
	if rse.options.Direction == SyncFromRemote || rse.options.Direction == SyncBoth {
		for relPath, remoteFile := range remoteFiles {
			localFullPath := filepath.Join(localBase, relPath)
			remoteFullPath := filepath.Join(remoteBase, relPath)

			localFile, exists := localFiles[relPath]
			shouldDownload := false

			if !exists {
				shouldDownload = true
			} else if rse.config.Incremental {
				// 增量同步：比较修改时间和大小
				if remoteFile.ModTime.After(localFile.ModTime) || remoteFile.Size != localFile.Size {
					shouldDownload = true
				}
			} else {
				shouldDownload = rse.options.Force
			}

			if shouldDownload {
				plan.Download = append(plan.Download, SyncItem{
					LocalPath:  localFullPath,
					RemotePath: remoteFullPath,
					Size:       remoteFile.Size,
					Action:     "download",
				})
			}
		}
	}

	// 检查需要删除的文件
	if rse.options.DeleteExtra {
		if rse.options.Direction == SyncToRemote || rse.options.Direction == SyncBoth {
			// 删除远程多余的文件
			for relPath, remoteFile := range remoteFiles {
				if _, exists := localFiles[relPath]; !exists {
					remoteFullPath := filepath.Join(remoteBase, relPath)
					plan.Delete = append(plan.Delete, SyncItem{
						RemotePath: remoteFullPath,
						Size:       remoteFile.Size,
						Action:     "delete_remote",
					})
				}
			}
		}
	}

	return plan
}

// executeSyncPlan 执行同步计划
func (rse *RemoteSyncEngine) executeSyncPlan(plan *SyncPlan, result *SyncResult) error {
	totalItems := len(plan.Upload) + len(plan.Download) + len(plan.Delete)

	var bar *pb.ProgressBar
	if rse.options.Progress && totalItems > 0 {
		bar = pb.StartNew(totalItems)
		defer bar.Finish()
	}

	// 执行上传
	for _, item := range plan.Upload {
		if bar != nil {
			bar.Increment()
		}

		if rse.options.Verbose {
			fmt.Printf("上传: %s -> %s\n", item.LocalPath, item.RemotePath)
		}

		if err := rse.sshClient.UploadFile(item.LocalPath, item.RemotePath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("上传失败 %s: %v", item.LocalPath, err))
			result.ErrorFiles++
		} else {
			result.UploadedFiles++
			result.TotalSize += item.Size
		}
	}

	// 执行下载
	for _, item := range plan.Download {
		if bar != nil {
			bar.Increment()
		}

		if rse.options.Verbose {
			fmt.Printf("下载: %s -> %s\n", item.RemotePath, item.LocalPath)
		}

		if err := rse.sshClient.DownloadFile(item.RemotePath, item.LocalPath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("下载失败 %s: %v", item.RemotePath, err))
			result.ErrorFiles++
		} else {
			result.DownloadFiles++
			result.TotalSize += item.Size
		}
	}

	// 执行删除
	for _, item := range plan.Delete {
		if bar != nil {
			bar.Increment()
		}

		if rse.options.Verbose {
			fmt.Printf("删除: %s\n", item.RemotePath)
		}

		command := fmt.Sprintf("rm -f '%s'", item.RemotePath)
		if _, err := rse.sshClient.ExecuteCommand(command); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("删除失败 %s: %v", item.RemotePath, err))
			result.ErrorFiles++
		} else {
			result.DeletedFiles++
		}
	}

	return nil
}

// printSyncPlan 打印同步计划
func (rse *RemoteSyncEngine) printSyncPlan(plan *SyncPlan) {
	fmt.Println("=== 同步计划 (试运行) ===")

	if len(plan.Upload) > 0 {
		fmt.Println("\n需要上传的文件:")
		for _, item := range plan.Upload {
			fmt.Printf("  上传: %s -> %s (%d bytes)\n", item.LocalPath, item.RemotePath, item.Size)
		}
	}

	if len(plan.Download) > 0 {
		fmt.Println("\n需要下载的文件:")
		for _, item := range plan.Download {
			fmt.Printf("  下载: %s -> %s (%d bytes)\n", item.RemotePath, item.LocalPath, item.Size)
		}
	}

	if len(plan.Delete) > 0 {
		fmt.Println("\n需要删除的文件:")
		for _, item := range plan.Delete {
			fmt.Printf("  删除: %s\n", item.RemotePath)
		}
	}

	fmt.Printf("\n总计: 上传 %d, 下载 %d, 删除 %d\n",
		len(plan.Upload), len(plan.Download), len(plan.Delete))
}

// ensureRemoteDirectory 确保远程目录存在
func (rse *RemoteSyncEngine) ensureRemoteDirectory(remotePath string) error {
	command := fmt.Sprintf("mkdir -p '%s'", remotePath)
	_, err := rse.sshClient.ExecuteCommand(command)
	return err
}
