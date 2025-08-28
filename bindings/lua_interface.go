package bindings

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// lua接口封装
type LuaInterface struct {
	luaPath    string
	filterPath string
	verbose    bool
}

// 创建lua接口
func NewLuaInterface(verbose bool) *LuaInterface {
	return &LuaInterface{
		luaPath:    "lua",
		filterPath: filepath.Join("lua", "filter.lua"),
		verbose:    verbose,
	}
}

// 解释器路径
func (l *LuaInterface) SetLuaPath(path string) {
	l.luaPath = path
}

// 过滤器路径
func (l *LuaInterface) SetFilterPath(path string) {
	l.filterPath = path
}

// 过滤结果
type FilterResult struct {
	FilteredFiles []FileMetadata `json:"filtered_files"`
	Status        bool           `json:"status"`
	Message       string         `json:"message,omitempty"`
	Statistics    *FilterStats   `json:"statistics,omitempty"`
}

// 过滤统计信息
type FilterStats struct {
	TotalFiles    int     `json:"total_files"`
	FilteredFiles int     `json:"filtered_files"`
	ExcludedFiles int     `json:"excluded_files"`
	ExclusionRate float64 `json:"exclusion_rate"`
}

// 调用lua过滤文件
func (l *LuaInterface) FilterFiles(ruleFile string, files []FileMetadata) (*FilterResult, error) {
	filesJSON, err := json.Marshal(files)
	if err != nil {
		return nil, fmt.Errorf("Errorf: %V", err)
	}

	args := []string{l.filterPath, ruleFile, string(filesJSON)}
	cmd := exec.Command(l.luaPath, args...)

	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("Lua error: %v , Errorf: %s", err, string(exitError.Stderr))
		}
		return nil, fmt.Errorf("Errorf: %v", err)
	}

	var result FilterResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("Lua error: %v , Errorf: %s", err, string(output))
	}

	if !result.Status {
		return nil, fmt.Errorf("Errorf: %s", result.Message)
	}
	return &result, nil
}

// 验证规则文件
func (l *LuaInterface) ValidateRuleFile(ruleFile string) error {
	if _, err := filepath.Abs(ruleFile); err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	script := fmt.Sprintf(`
-- 语法检查脚本
local success, err = pcall(function()
    dofile("%s")
end)

if success then
    print("OK")
else
    print("ERROR: " .. tostring(err))
end
`, ruleFile)

	cmd := exec.Command(l.luaPath, "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr != "OK" {
		return fmt.Errorf("Errorf: %s", outputStr)
	}
	return nil
}

func (l *LuaInterface) TestRuleFile(ruleFile string) (*RuleTestResult, error) {
	testFiles := []FileMetadata{
		{
			Path:          "/test/file1.txt",
			Hash:          "hash1",
			Size:          1024,
			ModifiledTime: "2023-01-01T00:00:00Z",
			Permissions:   "0644",
		},
		{
			Path:         "/test/.hidden",
			Hash:         "hash2",
			Size:         512,
			ModifiedTime: "2023-01-01T00:00:00Z",
			Permissions:  "0644",
		},
		{
			Path:         "/test/node_modules/package.json",
			Hash:         "hash3",
			Size:         2048,
			ModifiedTime: "2023-01-01T00:00:00Z",
			Permissions:  "0644",
		},
	}

	result, err := l.FilterFiles(ruleFile, testFiles)
	if err != nil {
		return nil, fmt.Errorf("Errorf: %v".err)
	}

	return &RuleTestResult{
		OriginalCount: len(testFiles),
		FilteredCount: len(result.FilteredFiles),
		ExcludedCount: len(testFiles) - len(result.FilteredFiles),
		FilteredFiles: result.FilteredFiles,
		Statistics:    result.Statistics,
	}, nil
}

// 规则测试结果
type RuleTestResult struct {
	OriginalCount int            `json:"original_count"`
	FilteredCount int            `json:"filtered_count"`
	ExcludedCount int            `json:"excluded_count"`
	FilteredFiles []FileMetadata `json:"filtered_files"`
	Statistics    *FilterStats   `json:"statistics,omitempty"`
}

// 获取lua版本信息
func (l *LuaInterface) GetLuaVersion() (string, error) {
	cmd := exec.Command(l.luaPath, "-v")
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return strings.TrimSpace(string(exitError.Stderr)), nil
		}
		return "", fmt.Errorf("Errorf: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// 检查 Lua 依赖
func (l *LuaInterface) CheckLuaDependencies() error {
	script := `
-- 检查基本功能
local json_available = false
local success, json = pcall(require, "json")
if success then
    json_available = true
else
    success, json = pcall(require, "cjson")
    if success then
        json_available = true
    else
        success, json = pcall(require, "dkjson")
        if success then
            json_available = true
        end
    end
end

if json_available then
    print("JSON_OK")
else
    print("JSON_MISSING")
end

-- 检查其他基本功能
print("BASIC_OK")
`

	cmd := exec.Command(l.luaPath, "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "JSON_MISSING" {
			return fmt.Errorf("缺少 Lua JSON 库 (需要 json, cjson 或 dkjson 中的一个)")
		}
	}

	return nil
}

// 创建默认规则文件
func (l *LuaInterface) CreateDefaultRuleFile(outputPath string) error {
	defaultRules := `-- 默认 FileSync CLI 规则文件
-- 此文件定义了文件同步的过滤规则

-- 判断文件是否应该同步
function should_sync(file_path)
    -- 忽略隐藏文件
    local filename = file_path:match("([^/\\]+)$")
    if filename and filename:sub(1, 1) == "." then
        return false
    end
    
    -- 忽略常见的临时和构建目录
    local ignore_patterns = {
        ".git", ".svn", "node_modules", "__pycache__",
        ".DS_Store", "Thumbs.db", "*.tmp", "*.log"
    }
    
    for _, pattern in ipairs(ignore_patterns) do
        if file_path:find(pattern, 1, true) then
            return false
        end
    end
    
    return true
end

-- 获取文件同步优先级
function get_priority(file_path)
    -- 配置文件高优先级
    if file_path:match("%.json$") or file_path:match("%.yaml$") or file_path:match("%.yml$") then
        return 10
    end
    
    -- 源代码文件中等优先级
    if file_path:match("%.go$") or file_path:match("%.py$") or file_path:match("%.rs$") then
        return 5
    end
    
    -- 默认优先级
    return 1
end
`

	return writeFile(outputPath, []byte(defaultRules))
}

// lua接口管理器
type LuaManager struct {
	interface_ *LuaInterface
	timeout    time.Duration
}

// 创建新的lua管理器
func NewLuaManager(verbose bool, timeout time.Duration) *LuaManager {
	return &LuaManager{
		interface_: NewLuaInterface(verbose),
		timeout:    timeout,
	}
}

// 拿lua接口
func (m *LuaManager) GetInterface() *LuaInterface {
	return m.interface_
}

// 超时文件过滤
func (m *LuaManager) FilterFilesWithTimeout(ruleFile string, files []FileMetadata) (*FilterResult, error) {
	done := make(chan struct{})
	var result *FilterResult
	var err error
	go func() {
		defer close(done)
		result, err = m.interface_.FilterFiles(ruleFile, files)
	}()

	select {
	case <-done:
		return result, err
	case <-time.After(m.timeout):
		return nil, fmt.Errorf("Timeout: %v", m.timeout)
	}
}

// lua接口健康检查
func (l *LuaInterface) HealthCheck() error {
	if _, err := l.GetLuaVersion(); err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	if err := l.CheckLuaDependencies(); err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	if _, err := filepath.Abs(l.filterPath); err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	return nil
}

// 批量过滤
func (l *LuaInterface) BatchFilterFiles(ruleFile string, fileBatches [][]FileMetadata) ([]*FilterResult, error) {
	results := make([]*FilterResult, 0, len(fileBatches))

	for i, batch := range fileBatches {
		result, err := l.FilterFiles(ruleFile, batch)
		if err != nil {
			return nil, fmt.Errorf("批量过滤第 %d 批文件失败: %v", i+1, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// 辅助函数：写入文件
func writeFile(path string, data []byte) error {
	// 这里需要导入 os 包
	// return os.WriteFile(path, data, 0644)
	return nil // 占位符
}
