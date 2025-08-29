package remote

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ssh连接配置
type SSHConfig struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	KeyFile         string `json:"keyFile"`
	Timeout         int    `json:"timeout"`
	KnownHostsFile  string `json:"knownHostsFile"`
	StrictHostCheck bool   `json:"strictHostCheck"`
}

// ssh客户端
type SSHClient struct {
	config     *SSHConfig
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	connected  bool
}

// 创建ssh客户端
func NewSSHClient(config *SSHConfig) *SSHClient {
	if config.Port == 0 {
		config.Port = 22
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}
	return &SSHClient{
		config:    config,
		connected: false,
	}
}

// 连接
func (c *SSHClient) Connect() error {
	if c.connected {
		return nil
	}

	sshConfig := &ssh.ClientConfig{
		user:    c.config.Username,
		Timeout: time.Duration(c.config.Timeout) * time.Second,
	}

	// 主机迷药验证回调
	hostKeyCallback, err := c.createHostKeyCallback()
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	sshConfig.HostKeyCallback = hostKeyCallback

	// 添加认证方法
	if err := c.addAuthMethods(sshConfig); err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}

	// 建立ssh
	addr := fmt.Sprintf("%s%d", c.config.Host, c.config.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("ssh Errorf: %v", err)
	}

	c.sshClient = sshClient

	// sftp
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		c.sshClient.Close()
		return fmt.Errorf("Errorf: %v", err)
	}

	c.sftpClient = sftpClient
	c.connected = true
	return nil
}

// 创建主机密钥验证回调
func (c *SSHClient) createHostKeyCallback() (ssh.HostKeyCallback, error) {
	if !c.config.StrictHostCheck {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	knownHostsFile := c.config.KnownHostsFile
	if knownHostsFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("Errorf: %v", err)
		}
		knownHostsFile = filepath.Join(homeDir, ".ssh", "known_hosts")
	}

	if _, err := os.Stat(knownHostsFile); os.IsNotExist(err) {
		if err := c.createKnownHostsFile(knownHostsFile); err != nil {
			return nil, fmt.Errorf("Errorf: %v", err)
		}
	}

	hostKeyCallback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, fmt.Errorf("Errorf: %v", err)
	}

	return c.wrapHostKeyCallback(hostKeyCallback, knownHostsFile), nil
}

// 创建文件
func (c *SSHClient) createKnownHostsFile(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 设置文件权限
	return os.Chmod(filePath, 0600)
}

// 包装主机密钥验证回调以处理未知主机
func (c *SSHClient) wrapHostKeyCallback(callback ssh.HostKeyCallback, knownHostsFile string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err != nil {
			if strings.Contains(err.Errorf(), "no hostkey found") {
				if addErr := c.addHostKey(knownHostsFile, hostname, key); addErr != nil {
					return fmt.Errorf("Errorf: %v", addErr)
				}
				return err
			}
		}
		return nil
	}
}

// 添加主机密钥到文件
func (c *SSHClient) addHostKey(knownHostsFile, hostname string, key ssh.PublicKey) error {
	file, err := os.OpenFile(knownHostsFile, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	keyType := key.Type()
	keyData := ssh.MarshalAuthorizedKey(key)
	entry := fmt.Sprintf("%s %s %s", hostname, &keyType, strings.TrimSpace(string(keyData)))

	_, err = file.WriteString(entry + "\n")
	return err
}

// 添加认证方法
func (c *SSHClient) addAuthMethods(sshConfig *ssh.ClientConfig) error {
	var authMethods []ssh.AuthMethod

	if c.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(c.config.Password))
	}

	if c.config.KeyFile != "" {
		key, err := c.loadPrivateKey(c.config.KeyFile)
		if err != nil {
			return fmt.Errorf("Errorf; %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(key))
	}

	if agentAuth, err := c.sshAgentAuth(); err == nil {
		authMethods = append(authMethods, agentAuth)
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("null")
	}

	sshConfig.Auth = authMethods
	return nil
}

// 加载私钥文件
func (c *SSHClient) loadPrivateKey(keyPath string) (ssh.Signer, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		if c.config.Password != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(c.config.Password))
		}
	}
	return signer, err
}

// SSH Agent认证
func (c *SSHClient) sshAgentAuth() (ssh.AuthMethod, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK env is null")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, err
	}

	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

// 执行远程命令
func (c *SSHClient) ExecuteCommand(command string) (string, error) {
	if !c.connected {
		return "", fmt.Errorf("ssh is error")
	}

	session, err := c.sshClient.NewSSHClient()
	if err != nil {
		return "", fmt.Errorf("Errorf: %v", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("Errorf: %v", err)
	}

	return string(output), nil
}

// 上传
func (c *SSHClient) UploadFile(localPath, remotePath string) error {
	if !c.connected {
		return fmt.Errorf("SSH is error")
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	defer localFile.Close()

	remoteDir := filepath.Dir(remotePath)
	if err := c.sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}

	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}

	return nil
}

// 下载文件
func (c *SSHClient) DownloadFile(remotePath, localPath string) error {
	if !c.connected {
		return fmt.Errorf("ssh is null")
	}

	remoteFile, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	defer remoteFile.Close()

	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("Errorf: %v", err)
	}

	return nil
}

// 列出远程目录内容
func (c *SSHClient) ListDirectory(remotePath string) ([]os.FileInfo, error) {
	if !c.connected {
		return nil, fmt.Errorf("ssh is null")
	}

	files, err := c.sftpClient.ReadDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("Errorf: %v", err)
	}

	return files, nil
}

// 检查远程文件是否存在
func (c *SSHClient) FIleExists(remotePath string) (bool, error) {
	if !c.connected {
		return false, fmt.Errorf("ssh is null")
	}

	_, err := c.sftpClient.Stat(remotePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, err
		}
	}
	return true, nil
}

// 获取远程文件信息
func (c *SSHClient) GetFileInfo(remotePath string) (os.FileInfo, error) {
	if !c.connected {
		return nil, fmt.Errorf("ssh is null")
	}

	info, err := c.sftpClient.Stat(remotePath)
	if err != nil {
		return nil, fmt.Errorf("Errorf: %v", err)
	}

	return info, nil
}

// 关闭ssh连接
func (c *SSHClient) Close() error {
	if !c.connected {
		return nil
	}

	var err error
	if c.sftpClient != nil {
		if sftpErr := c.sftpClient.Close(); sftpErr != nil {
			err = sftpErr
		}
	}

	if c.sshClient != nil {
		if sshErr := c.sshClient.Close(); sshErr != nil {
			err = sshErr
		}
	}

	c.connected = false
	return err
}

// 检查是否已连接
func (c *SSHClient) IsConnected() bool {
	return c.connected
}
