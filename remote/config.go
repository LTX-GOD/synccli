package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// 远程同步配置
type RemoteConfig struct {
	Name            string     `json:"name"`
	SSH             *SSHConfig `json:"ssh"`
	RemoteBase      string     `json:"remoteBase"`
	Compression     bool       `json:"compression"`
	Encryption      bool       `json:"encryption"`
	Incremental     bool       `json:"incremental"`
	KnownHostsFile  string     `json:"knownHostsFile"`
	StrictHostCheck bool       `json:"strictHostCheck"`
	ExcludeList     []string   `json:"excludeList"`
}

// 配置管理器
type ConfigManager struct {
	configDir  string
	configFile string
	configs    map[string]*RemoteConfig
}

// 创建新的配置管理器
func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("Errorf: %v", err)
	}

	configDir := filepath.Join(homeDir, ".synccli")
	configFile := filepath.Join(configDir, "remote_configs.json")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("Errorf: %v", err)
	}

	cm := &ConfigManager{
		configDir:  configDir,
		configFile: configFile,
		configs:    make(map[string]*RemoteConfig),
	}

	if err := cm.LoadConfigs(); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return cm, nil
}

// 加载配置文件
func (cm *ConfigManager) LoadConfigs() error {
	data, err := os.ReadFile(cm.configFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &cm.configs)
}

// 保存配置文件
func (cm *ConfigManager) SaveConfigs() error {
	data, err := json.MarshalIndent(cm.configs, "", " ")
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	return os.WriteFile(cm.configFile, data, 0600)
}

// 添加远程配置
func (cm *ConfigManager) AddConfig(config *RemoteConfig) error {
	if config.Name == "" {
		return fmt.Errorf("the name is null")
	}
	if config.SSH == nil {
		return fmt.Errorf("ssh is null")
	}
	if config.SSH.Host == "" {
		return fmt.Errorf("the host is null")
	}
	if config.SSH.Username == "" {
		return fmt.Errorf("the name is null")
	}
	if config.RemoteBase == "" {
		config.RemoteBase = "/tmp/synccli"
	}

	cm.configs[config.Name] = config
	return cm.SaveConfigs()
}

// 获取指定名称的配置
func (cm *ConfigManager) GetConfig(name string) (*RemoteConfig, error) {
	config, err := cm.configs[name]
	if !err {
		return nil, fmt.Errorf("this is null: %s", name)
	}
	return config, nil
}

// 列出所有配置
func (cm *ConfigManager) ListConfigs() map[string]*RemoteConfig {
	return cm.configs
}

// 删除指定配置
func (cm *ConfigManager) RemoveConfig(name string) error {
	if _, exists := cm.configs[name]; !exists {
		return fmt.Errorf("this is null: %s", name)
	}

	delete(cm.configs, name)
	return cm.SaveConfigs()
}

// 更新配置
func (cm *ConfigManager) UpdateConfig(name string, config *RemoteConfig) error {
	if _, exists := cm.configs[name]; !exists {
		return fmt.Errorf("this is null: %s", name)
	}
	config.Name = name
	cm.configs[name] = config
	return cm.SaveConfigs()
}

// 验证配置
func (cm *ConfigManager) ValidateConfig(config *RemoteConfig) error {
	if config.SSH == nil {
		return fmt.Errorf("ssh is null")
	}
	if config.SSH.Host == "" {
		return fmt.Errorf("the host is null")
	}
	if config.SSH.Username == "" {
		return fmt.Errorf("the name is null")
	}
	if config.SSH.Password == "" && config.SSH.KeyFile == "" {
		return fmt.Errorf("the password or key is null")
	}

	if config.SSH.KeyFile != "" {
		if _, err := os.Stat(config.SSH.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("the key is null: %s", config.SSH.KeyFile)
		}
	}
	return nil
}

// 默认配置模板
func (cm *ConfigManager) CreateDefaultConfig(name, host, username string) *RemoteConfig {
	return &RemoteConfig{
		Name: name,
		SSH: &SSHConfig{
			Host:     host,
			Port:     22,
			Username: username,
			Timeout:  30,
		},
		RemoteBase:      "/tmp/synccli",
		Compression:     true,
		Encryption:      true,
		Incremental:     true,
		KnownHostsFile:  "",
		StrictHostCheck: true,
		ExcludeList: []string{
			".git",
			".DS_Store",
			"*.tmp",
			"*.log",
			"node_modules",
			"__pycache__",
			"target",
		},
	}
}
