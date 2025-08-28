package bindings

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// python接口封装
type PythonInterface struct {
	pythonPath string
	scriptPath string
	verbose    bool
}

// 创建新的python接口
func NewPythonInterface(verbose bool) *PythonInterface {
	return &PythonInterface{
		pythonPath: "python3",
		scriptPath: filepath.Join("python", "scanner.py"),
		verbose:    verbose,
	}
}

// python解释器路径
func (p *PythonInterface) SetPythonPath(path string) {
	p.pythonPath = path
}

func (p *PythonInterface) SetScriptPath(path string) {
	p.scriptPath = path
}

// 扫描结果
type ScanResult struct {
	SourceFiles []FileMetadata `json:"source_files"`
	DestFiles   []FileMetadata `json:"dest_files"`
	Status      bool           `json:"status"`
	Message     string         `json:"message,omitempty"`
	Statistics  *ScanStats     `json:"statistics,omitempty"`
}

// 扫描统计信息
type ScanStats struct {
	Source ScanStatDetail `json:"source"`
	Dest   ScanStatDetail `json:"dest"`
}

// 统计详情
type ScanStatDetail struct {
	ScannedFiles int     `json:"scanned_files"`
	TotalSize    int64   `json:"total_size"`
	TotalSizeMB  float64 `json:"total_size_mb"`
}

// 调用python扫描
func (p *PythonInterface) ScanDirectories(sourcePath, destPath string) (*ScanResult, error) {
	args := []string{p.scriptPath, sourcePath, destPath}
	if p.verbose {
		args = append(args, "--verbose")
	}
	cmd := exec.Command(p.pythonPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("Errorf: %v , print: %s", err, string(exitError.Stderr))
		}
		return nil, fmt.Errorf("Errorf: %v", err)
	}

	var result ScanResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("Errorf: %v , print: %s", err, string(output))
	}

	if !result.Status {
		return nil, fmt.Errorf("Errorf: %s", result.Message)
	}
	return &result, nil
}

// 计算hash
func (p *PythonInterface) CalculateFileHash(filepath string) (string, error) {
	script := fmt.Sprintf(`
import sys
import hashlib
import json

def calculate_hash(file_path):
    try:
        hash_sha256 = hashlib.sha256()
        with open(file_path, "rb") as f:
            for chunk in iter(lambda: f.read(4096), b""):
                hash_sha256.update(chunk)
        return hash_sha256.hexdigest()
    except Exception as e:
        return None

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(json.dumps({"success": False, "message": "用法: python script.py <文件路径>"}))
        sys.exit(1)
    
    file_path = sys.argv[1]
    hash_value = calculate_hash(file_path)
    
    if hash_value:
        print(json.dumps({"success": True, "hash": hash_value}))
    else:
        print(json.dumps({"success": False, "message": "计算哈希失败"}))
		`)
	cmd := exec.Command(p.pythonPath, "-c", script, filepath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Hash Errorf: %v", err)
	}

	var result struct {
		Success bool   `json:"success"`
		Hash    string `json:"hash,omitempty"`
		Message string `json:"message,omitempty"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("Hash Errorf: %v", err)
	}

	if !result.Success {
		return "", fmt.Errorf("Hash Errorf: %s", result.Message)
	}

	return result.Hash, nil
}

// 验证目录是否可以访问
func (p *PythonInterface) ValidateDirectories(paths ...string) error {
	for _, path := range paths {
		script := fmt.Sprintf(`
import os
import sys
import json

path = sys.argv[1]
if os.path.exists(path) and os.path.isdir(path):
    print(json.dumps({"success": True, "message": "目录有效"}))
else:
    print(json.dumps({"success": False, "message": "目录不存在或不可访问"}))
			`)
		cmd := exec.Command(p.pythonPath, "-c", script, path)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("The path %s error: %v", path, err)
		}

		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}

		if err := json.Unmarshal(output, &result); err != nil {
			return fmt.Errorf("Errorf: %v", err)
		}

		if !result.Success {
			return fmt.Errorf("The path %s error: %s", path, result.Message)
		}
	}
	return nil
}

// python版本信息
func (p *PythonInterface) GetPythonVersion() (string, error) {
	cmd := exec.Command(p.pythonPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Errorf: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// 检查 Python 依赖是否安装
func (p *PythonInterface) CheckPythonDependencies() error {
	requiredModules := []string{"hashlib", "json", "os", "sys", "pathlib"}

	for _, module := range requiredModules {
		script := fmt.Sprintf(`
try:
    import %s
    print("OK")
except ImportError:
    print("MISSING")
`, module)

		cmd := exec.Command(p.pythonPath, "-c", script)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("检查模块 %s 失败: %v", module, err)
		}

		if strings.TrimSpace(string(output)) != "OK" {
			return fmt.Errorf("缺少 Python 模块: %s", module)
		}
	}

	return nil
}

// 接口管理器
type PythonManager struct {
	interface_ *PythonInterface
	timeout    time.Duration
}

// 创建新的 Python 管理器
func NewPythonManager(verbose bool, timeout time.Duration) *PythonManager {
	return &PythonManager{
		interface_: NewPythonInterface(verbose),
		timeout:    timeout,
	}
}

// 获取 Python 接口
func (m *PythonManager) GetInterface() *PythonInterface {
	return m.interface_
}

// 带超时的目录扫描
func (m *PythonManager) ScanDirectoriesWithTimeout(sourcePath, destPath string) (*ScanResult, error) {
	// 创建带超时的上下文
	done := make(chan struct{})
	var result *ScanResult
	var err error

	go func() {
		defer close(done)
		result, err = m.interface_.ScanDirectories(sourcePath, destPath)
	}()

	select {
	case <-done:
		return result, err
	case <-time.After(m.timeout):
		return nil, fmt.Errorf("Python 扫描超时 (%v)", m.timeout)
	}
}

// 接口健康检查
func (p *PythonInterface) HealthCheck() error {
	// 检查 Python 是否可用
	if _, err := p.GetPythonVersion(); err != nil {
		return fmt.Errorf("Python 不可用: %v", err)
	}

	// 检查依赖
	if err := p.CheckPythonDependencies(); err != nil {
		return fmt.Errorf("Python 依赖检查失败: %v", err)
	}

	// 检查脚本文件是否存在
	if _, err := filepath.Abs(p.scriptPath); err != nil {
		return fmt.Errorf("Python 脚本路径无效: %v", err)
	}

	return nil
}

// 批量扫描多个目录对
func (p *PythonInterface) BatchScanDirectories(dirPairs []struct{ Source, Dest string }) ([]*ScanResult, error) {
	results := make([]*ScanResult, 0, len(dirPairs))

	for i, pair := range dirPairs {
		result, err := p.ScanDirectories(pair.Source, pair.Dest)
		if err != nil {
			return nil, fmt.Errorf("批量扫描第 %d 对目录失败: %v", i+1, err)
		}
		results = append(results, result)
	}

	return results, nil
}
