package infrastructure

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	sshClient       *ssh.Client
	sftpClient      *sftp.Client
	config          *config.SyncCfg
	keepAliveCtx    context.Context
	keepAliveCancel context.CancelFunc
	keepAliveWg     sync.WaitGroup
	mu              sync.RWMutex
	connected       bool
	hostKeyChecker  *HostKeyChecker
}

type SSHClientInterface interface {
	Connect(server *domain.Server) error
	Close() error
	IsConnected() bool
	ListFiles(remotePath string) ([]string, error)
	DownloadResumable(remotePath, localPath string) error
	DownloadWithRetry(remotePath, localPath string) error
	DownloadWithRetryProgress(remotePath, localPath string, progressCallback func(downloaded, total int64)) error
	UploadResumable(localPath, remotePath string) error
	UploadWithRetry(localPath, remotePath string) error
	UploadWithRetryProgress(localPath, remotePath string, progressCallback func(uploaded, total int64)) error // добавить
	DeleteFile(remotePath string) error
	GetFileSize(remotePath string) (int64, error)
	CreateEmptyFile(remotePath string) (*sftp.File, error)
}

type HostKeyChecker struct {
	knownHostsPath string
	autoAddEnabled bool
}

func NewHostKeyChecker(knownHostsPath string) *HostKeyChecker {
	logger.Get().Info().Str("input_path", knownHostsPath).Msg("NewHostKeyChecker called")

	if knownHostsPath == "" {
		knownHostsPath = "./data/ssh/known_hosts"
		logger.Get().Info().Str("resolved_path", knownHostsPath).Msg("Using default path")
	}

	sshDir := filepath.Dir(knownHostsPath)
	logger.Get().Info().Str("ssh_dir", sshDir).Msg("Creating .ssh directory")

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		logger.Get().Warn().Err(err).Msg("Failed to create .ssh directory")
	} else {
		logger.Get().Info().Str("ssh_dir", sshDir).Msg(".ssh directory created")
	}

	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		logger.Get().Info().Str("path", knownHostsPath).Msg("known_hosts file does not exist, creating")
		file, err := os.Create(knownHostsPath)
		if err != nil {
			logger.Get().Warn().Err(err).Msg("Failed to create known_hosts file")
		} else {
			file.Close()
			logger.Get().Info().Str("path", knownHostsPath).Msg("Created new known_hosts file")
		}
	} else {
		logger.Get().Info().Str("path", knownHostsPath).Msg("known_hosts file already exists")
	}

	return &HostKeyChecker{
		autoAddEnabled: true,
		knownHostsPath: knownHostsPath,
	}
}

func (c *HostKeyChecker) EnableAutoAdd(enabled bool) {
	c.autoAddEnabled = enabled
}

func (c *HostKeyChecker) CheckHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	found, matched := c.findHostKey(hostname, key)

	if found && matched {
		return nil
	}

	if found && !matched {
		logger.Get().Error().
			Str("hostname", hostname).
			Str("fingerprint", ssh.FingerprintSHA256(key)).
			Msg("HOST KEY MISMATCH! Connection rejected")
		return fmt.Errorf("host key mismatch for %s", hostname)
	}

	if !found && c.autoAddEnabled {
		logger.Get().Info().
			Str("hostname", hostname).
			Msg("First connection - adding host key to known_hosts")

		if err := c.addHostKey(hostname, key); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("hostname", hostname).
				Msg("Failed to add host key")
			return err
		}

		logger.Get().Info().
			Str("hostname", hostname).
			Msg("Host key added - future connections will be trusted")
		return nil
	}

	return fmt.Errorf("host key not found for %s", hostname)
}

func (c *HostKeyChecker) findHostKey(hostname string, key ssh.PublicKey) (bool, bool) {
	file, err := os.Open(c.knownHostsPath)
	if err != nil {
		return false, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	keyStr := base64.StdEncoding.EncodeToString(key.Marshal())

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		hostPatterns := strings.Split(fields[0], ",")
		hostMatched := false
		for _, pattern := range hostPatterns {
			if c.matchHost(pattern, hostname) {
				hostMatched = true
				break
			}
		}
		if !hostMatched {
			continue
		}

		if len(fields) >= 3 && fields[2] == keyStr {
			return true, true
		}
		return true, false
	}

	return false, false
}

func (c *HostKeyChecker) addHostKey(hostname string, key ssh.PublicKey) error {
	sshDir := filepath.Dir(c.knownHostsPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	file, err := os.OpenFile(c.knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open known_hosts: %w", err)
	}
	defer file.Close()

	keyLine := fmt.Sprintf("%s %s %s\n", hostname, key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))

	if _, err := file.WriteString(keyLine); err != nil {
		return fmt.Errorf("failed to write to known_hosts: %w", err)
	}

	return nil
}

func (c *HostKeyChecker) matchHost(pattern, hostname string) bool {
	if pattern == hostname {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if strings.HasPrefix(hostname, parts[0]) && strings.HasSuffix(hostname, parts[len(parts)-1]) {
			return true
		}
	}
	return false
}

func NewSSHClient(cfg *config.SyncCfg) *SSHClient {
	if cfg == nil {
		cfg = &config.SyncCfg{
			SSHConTimeout: 10 * time.Second,
			SSHKeepAlive:  30 * time.Second,
			RetryMaxAtmpt: 3,
			RetryDelay:    5 * time.Second,
		}
	}
	return &SSHClient{
		config:         cfg,
		hostKeyChecker: NewHostKeyChecker(""),
	}
}

func (c *SSHClient) Connect(server *domain.Server) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

	if c.config == nil {
		return fmt.Errorf("ssh client config is nil")
	}

	var methods []ssh.AuthMethod
	switch server.AuthType {
	case "password":
		if server.Password == nil {
			return fmt.Errorf("password is nil for password auth")
		}
		methods = []ssh.AuthMethod{ssh.Password(*server.Password)}
		logger.Get().Debug().
			Str("username", server.Username).
			Msg("Using password authentication")
	case "key":
		if server.PrivateKey == nil {
			return fmt.Errorf("private key is nil for key auth")
		}
		signer, err := ssh.ParsePrivateKey([]byte(*server.PrivateKey))
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		methods = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		logger.Get().Debug().
			Str("username", server.Username).
			Msg("Using key authentication")
	default:
		return fmt.Errorf("unsupported auth type: %s", server.AuthType)
	}

	sshConfig := &ssh.ClientConfig{
		User:            server.Username,
		Auth:            methods,
		HostKeyCallback: c.verifyHostKey,
		Timeout:         c.config.SSHConTimeout,
	}

	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)

	logger.Get().Info().
		Str("server", server.Name).
		Str("addr", addr).
		Msg("Connecting to SSH server")

	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial SSH: %w", err)
	}

	c.sshClient = sshClient

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		c.sshClient.Close()
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	c.sftpClient = sftpClient

	c.startKeepAlive(server.Name)
	c.connected = true

	logger.Get().Info().
		Str("server", server.Name).
		Msg("SSH connection established successfully")

	return nil
}

func (c *SSHClient) verifyHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	cleanHostname := hostname
	if idx := strings.Index(hostname, ":"); idx != -1 {
		cleanHostname = hostname[:idx]
	}

	logger.Get().Info().
		Str("hostname", cleanHostname).
		Str("known_hosts_path", c.hostKeyChecker.knownHostsPath).
		Msg("verifyHostKey called")

	return c.hostKeyChecker.CheckHostKey(cleanHostname, remote, key)
}

func (c *SSHClient) startKeepAlive(serverName string) {
	c.keepAliveCtx, c.keepAliveCancel = context.WithCancel(context.Background())

	c.keepAliveWg.Add(1)
	go func() {
		defer c.keepAliveWg.Done()

		interval := c.config.SSHKeepAlive
		if interval == 0 {
			interval = 30 * time.Second
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		logger.Get().Debug().
			Str("server", serverName).
			Dur("interval", interval).
			Msg("SSH Keep-Alive goroutine started")

		for {
			select {
			case <-c.keepAliveCtx.Done():
				logger.Get().Debug().
					Str("server", serverName).
					Msg("SSH Keep-Alive goroutine stopped")
				return

			case <-ticker.C:
				if err := c.sendKeepAlive(); err != nil {
					logger.Get().Warn().
						Str("server", serverName).
						Err(err).
						Msg("SSH Keep-Alive failed, connection may be dead")

					c.mu.Lock()
					if c.connected {
						c.connected = false
					}
					c.mu.Unlock()

					return
				}

				logger.Get().Trace().
					Str("server", serverName).
					Msg("SSH Keep-Alive sent successfully")
			}
		}
	}()
}

func (c *SSHClient) sendKeepAlive() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.sshClient == nil {
		return fmt.Errorf("SSH client is nil")
	}

	_, _, err := c.sshClient.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("connection closed: %w", err)
		}
		return fmt.Errorf("keepalive request failed: %w", err)
	}

	return nil
}

func (c *SSHClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	logger.Get().Debug().Msg("Closing SSH connection")

	if c.keepAliveCancel != nil {
		c.keepAliveCancel()
		c.keepAliveWg.Wait()
	}

	var errs []error

	if c.sftpClient != nil {
		if err := c.sftpClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("sftp close: %w", err))
		}
		c.sftpClient = nil
	}

	if c.sshClient != nil {
		if err := c.sshClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("ssh close: %w", err))
		}
		c.sshClient = nil
	}

	c.connected = false

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	logger.Get().Debug().Msg("SSH connection closed successfully")
	return nil
}

func (c *SSHClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.sshClient == nil {
		return false
	}

	_, _, err := c.sshClient.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		c.mu.RUnlock()
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		c.mu.RLock()
		return false
	}

	return true
}

func (c *SSHClient) reconnectAttempt(server *domain.Server) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	if c.sshClient != nil {
		_, _, err := c.sshClient.SendRequest("keepalive@openssh.com", true, nil)
		if err == nil {
			return nil
		}
	}

	logger.Get().Warn().
		Str("server", server.Name).
		Msg("SSH connection appears dead, attempting reconnect")

	if c.sftpClient != nil {
		c.sftpClient.Close()
		c.sftpClient = nil
	}
	if c.sshClient != nil {
		c.sshClient.Close()
		c.sshClient = nil
	}

	if c.keepAliveCancel != nil {
		c.keepAliveCancel()
		c.keepAliveWg.Wait()
	}

	c.connected = false

	var methods []ssh.AuthMethod
	switch server.AuthType {
	case "password":
		methods = []ssh.AuthMethod{ssh.Password(*server.Password)}
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(*server.PrivateKey))
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		methods = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	default:
		return fmt.Errorf("unsupported auth type: %s", server.AuthType)
	}

	sshConfig := &ssh.ClientConfig{
		User:            server.Username,
		Auth:            methods,
		HostKeyCallback: c.verifyHostKey,
		Timeout:         c.config.SSHConTimeout,
	}

	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)

	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("reconnect failed: %w", err)
	}
	c.sshClient = sshClient

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		c.sshClient.Close()
		return fmt.Errorf("failed to create SFTP client during reconnect: %w", err)
	}
	c.sftpClient = sftpClient

	c.startKeepAlive(server.Name)

	c.connected = true

	logger.Get().Info().
		Str("server", server.Name).
		Msg("SSH connection re-established successfully")

	return nil
}

func (c *SSHClient) ListFiles(remotePath string) ([]string, error) {
	c.mu.RLock()
	if c.sftpClient == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("not connected")
	}
	sftpClient := c.sftpClient
	c.mu.RUnlock()

	entries, err := sftpClient.ReadDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

func (c *SSHClient) DownloadResumable(remotePath, localPath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("not connected")
	}

	partPath := localPath + ".part"

	var offset int64 = 0
	if info, err := os.Stat(partPath); err == nil {
		offset = info.Size()
		if offset > 0 {
			logger.Get().Info().
				Str("file", remotePath).
				Int64("offset", offset).
				Msg("Resuming download from offset")
		}
	}

	remoteFile, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	remoteInfo, err := remoteFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat remote file: %w", err)
	}
	remoteSize := remoteInfo.Size()

	if offset >= remoteSize {
		if offset > remoteSize {
			logger.Get().Warn().
				Str("file", remotePath).
				Int64("part_size", offset).
				Int64("remote_size", remoteSize).
				Msg("Part file larger than remote, restarting download")
		} else {
			return os.Rename(partPath, localPath)
		}
	}

	if offset > 0 {
		_, err = remoteFile.Seek(offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek remote file: %w", err)
		}
	}

	localFile, err := os.OpenFile(partPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open local part file: %w", err)
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("download interrupted: %w", err)
	}

	if err := os.Rename(partPath, localPath); err != nil {
		return fmt.Errorf("failed to rename part file: %w", err)
	}

	return nil
}

func (c *SSHClient) UploadResumable(localPath, remotePath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("not connected")
	}

	remotePartPath := remotePath + ".part"

	var offset int64 = 0
	if info, err := c.sftpClient.Stat(remotePartPath); err == nil {
		offset = info.Size()
		if offset > 0 {
			logger.Get().Info().
				Str("file", localPath).
				Int64("offset", offset).
				Msg("Resuming upload from offset")
		}
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	localInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}
	localSize := localInfo.Size()

	if offset >= localSize {
		if offset > localSize {
			logger.Get().Warn().
				Str("file", localPath).
				Int64("part_size", offset).
				Int64("local_size", localSize).
				Msg("Part file larger than local, restarting upload")
		} else {
			return c.sftpClient.Rename(remotePartPath, remotePath)
		}
	}

	if offset > 0 {
		_, err = localFile.Seek(offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek local file: %w", err)
		}
	}

	var remoteFile *sftp.File
	if offset == 0 {
		remoteFile, err = c.sftpClient.Create(remotePartPath)
	} else {
		remoteFile, err = c.sftpClient.OpenFile(remotePartPath, os.O_APPEND|os.O_WRONLY)
	}
	if err != nil {
		return fmt.Errorf("failed to open remote part file: %w", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("upload interrupted: %w", err)
	}

	err = c.sftpClient.Rename(remotePartPath, remotePath)
	if err != nil {
		return fmt.Errorf("failed to rename remote part file: %w", err)
	}

	return nil
}

func (c *SSHClient) DeleteFile(remotePath string) error {
	if c.sftpClient == nil {
		return fmt.Errorf("not connected")
	}

	err := c.sftpClient.Remove(remotePath)
	if err != nil {
		return fmt.Errorf("failed to delete remote file: %w", err)
	}

	return nil
}

func (c *SSHClient) DownloadWithRetry(remotePath, localPath string) error {
	return c.DownloadWithRetryProgress(remotePath, localPath, nil)
}

func (c *SSHClient) DownloadWithRetryProgress(remotePath, localPath string, progressCallback func(downloaded, total int64)) error {
	var lastErr error
	baseDelay := 2 * time.Second

	for attempt := 0; attempt < c.config.RetryMaxAtmpt; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(attempt*attempt)
			logger.Get().Warn().
				Int("attempt", attempt+1).
				Int("max_attempts", c.config.RetryMaxAtmpt).
				Str("file", remotePath).
				Dur("delay", delay).
				Msg("Retrying download")
			time.Sleep(delay)
		}

		err := c.DownloadWithProgress(remotePath, localPath, progressCallback)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("download failed after %d attempts: %w", c.config.RetryMaxAtmpt, lastErr)
}

func (c *SSHClient) UploadWithRetry(localPath, remotePath string) error {
	return c.UploadWithRetryProgress(localPath, remotePath, nil)
}

func (c *SSHClient) GetFileSize(remotePath string) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.sftpClient == nil {
		return 0, fmt.Errorf("not connected")
	}

	info, err := c.sftpClient.Stat(remotePath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat remote file: %w", err)
	}

	return info.Size(), nil
}

func (c *SSHClient) DownloadWithProgress(remotePath, localPath string, progressCallback func(downloaded, total int64)) error {
	if c.sftpClient == nil {
		return fmt.Errorf("not connected")
	}

	if err := ensureLocalDir(localPath); err != nil {
		return err
	}

	remoteFile, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	remoteInfo, err := remoteFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat remote file: %w", err)
	}
	totalSize := remoteInfo.Size()

	finalLocalPath := getUniqueLocalPath(localPath)

	if totalSize == 0 {
		logger.Get().Debug().
			Str("file", remotePath).
			Msg("Empty file detected, creating empty local file")

		emptyFile, err := os.Create(finalLocalPath)
		if err != nil {
			return fmt.Errorf("failed to create empty local file: %w", err)
		}
		emptyFile.Close()

		if progressCallback != nil {
			progressCallback(0, 0)
		}

		logger.Get().Debug().
			Str("file", remotePath).
			Str("saved_as", filepath.Base(finalLocalPath)).
			Msg("Empty file downloaded successfully")
		return nil
	}

	partPath := finalLocalPath + ".part"

	var offset int64 = 0
	if info, err := os.Stat(partPath); err == nil {
		offset = info.Size()
	}

	if offset >= totalSize {
		if offset > totalSize {
			logger.Get().Warn().
				Str("file", remotePath).
				Int64("part_size", offset).
				Int64("remote_size", totalSize).
				Msg("Part file larger than remote, restarting download")
			offset = 0
			os.Remove(partPath)
		} else {
			return os.Rename(partPath, finalLocalPath)
		}
	}

	var localFile *os.File
	if offset > 0 {
		localFile, err = os.OpenFile(partPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		localFile, err = os.Create(partPath)
	}
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	if offset > 0 {
		_, err = remoteFile.Seek(offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek remote file: %w", err)
		}
	}

	buffer := make([]byte, 32*1024)
	downloaded := offset

	if progressCallback != nil {
		progressCallback(downloaded, totalSize)
	}

	for {
		n, err := remoteFile.Read(buffer)
		if n > 0 {
			_, writeErr := localFile.Write(buffer[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write to local file: %w", writeErr)
			}
			downloaded += int64(n)

			if progressCallback != nil && n > 0 && downloaded%(1024*1024) < int64(n) {
				progressCallback(downloaded, totalSize)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read remote file: %w", err)
		}
	}

	localFile.Close()

	if err := os.Rename(partPath, finalLocalPath); err != nil {
		return fmt.Errorf("failed to rename part file: %w", err)
	}

	if progressCallback != nil {
		progressCallback(totalSize, totalSize)
	}

	logger.Get().Info().
		Str("file", remotePath).
		Str("saved_as", filepath.Base(finalLocalPath)).
		Msg("File downloaded successfully")

	return nil
}

func (c *SSHClient) UploadWithProgress(localPath, remotePath string, progressCallback func(uploaded, total int64)) error {
	if c.sftpClient == nil {
		return fmt.Errorf("not connected")
	}

	localInfo, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", localPath)
		}
		return fmt.Errorf("failed to stat local file: %w", err)
	}
	totalSize := localInfo.Size()

	finalRemotePath := getUniqueRemotePath(c.sftpClient, remotePath)

	if totalSize == 0 {
		logger.Get().Debug().
			Str("file", localPath).
			Msg("Empty file detected, creating empty file on server")

		remoteFile, err := c.sftpClient.Create(finalRemotePath)
		if err != nil {
			return fmt.Errorf("failed to create empty remote file: %w", err)
		}
		remoteFile.Close()

		if progressCallback != nil {
			progressCallback(0, 0)
		}

		logger.Get().Info().
			Str("file", localPath).
			Str("uploaded_as", filepath.Base(finalRemotePath)).
			Msg("Empty file uploaded successfully")
		return nil
	}

	remotePartPath := finalRemotePath + ".part"

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	var offset int64 = 0
	if info, err := c.sftpClient.Stat(remotePartPath); err == nil {
		offset = info.Size()
		logger.Get().Debug().
			Str("file", localPath).
			Int64("offset", offset).
			Msg("Found existing part file on server")
	}

	if offset >= totalSize {
		if offset > totalSize {
			logger.Get().Warn().
				Str("file", localPath).
				Int64("part_size", offset).
				Int64("local_size", totalSize).
				Msg("Part file larger than local, restarting upload")
			c.sftpClient.Remove(remotePartPath)
			offset = 0
		} else {
			return c.sftpClient.Rename(remotePartPath, finalRemotePath)
		}
	}

	var remoteFile *sftp.File
	if offset > 0 {
		remoteFile, err = c.sftpClient.OpenFile(remotePartPath, os.O_APPEND|os.O_WRONLY)
	} else {
		remoteFile, err = c.sftpClient.Create(remotePartPath)
	}
	if err != nil {
		return fmt.Errorf("failed to open remote part file: %w", err)
	}
	defer remoteFile.Close()

	if offset > 0 {
		_, err = localFile.Seek(offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek local file: %w", err)
		}
	}

	buffer := make([]byte, 32*1024)
	uploaded := offset

	if progressCallback != nil {
		progressCallback(uploaded, totalSize)
	}

	for {
		n, err := localFile.Read(buffer)
		if n > 0 {
			_, writeErr := remoteFile.Write(buffer[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write to remote file: %w", writeErr)
			}
			uploaded += int64(n)

			if progressCallback != nil && uploaded%(1024*1024) < int64(n) {
				progressCallback(uploaded, totalSize)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read local file: %w", err)
		}
	}

	remoteFile.Close()

	if err := c.sftpClient.Rename(remotePartPath, finalRemotePath); err != nil {
		return fmt.Errorf("failed to rename remote part file: %w", err)
	}

	if progressCallback != nil {
		progressCallback(totalSize, totalSize)
	}

	logger.Get().Info().
		Str("file", localPath).
		Str("uploaded_as", filepath.Base(finalRemotePath)).
		Msg("Upload completed successfully")

	return nil
}

func (c *SSHClient) UploadWithRetryProgress(localPath, remotePath string, progressCallback func(uploaded, total int64)) error {
	var lastErr error
	baseDelay := 2 * time.Second

	for attempt := 0; attempt < c.config.RetryMaxAtmpt; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(attempt*attempt)
			logger.Get().Warn().
				Int("attempt", attempt+1).
				Int("max_attempts", c.config.RetryMaxAtmpt).
				Str("file", localPath).
				Dur("delay", delay).
				Msg("Retrying upload")
			time.Sleep(delay)
		}

		err := c.UploadWithProgress(localPath, remotePath, progressCallback)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("upload failed after %d attempts: %w", c.config.RetryMaxAtmpt, lastErr)
}

func ensureLocalDir(localPath string) error {
	dir := localPath
	if idx := strings.LastIndex(localPath, "/"); idx != -1 {
		dir = localPath[:idx]
	} else if idx := strings.LastIndex(localPath, "\\"); idx != -1 {
		dir = localPath[:idx]
	}

	if dir != localPath {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func getUniqueLocalPath(originalPath string) string {
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return originalPath
	}

	dir := filepath.Dir(originalPath)
	ext := filepath.Ext(originalPath)
	name := strings.TrimSuffix(filepath.Base(originalPath), ext)

	for i := 1; i <= 1000; i++ {
		newName := fmt.Sprintf("%s (%d)%s", name, i, ext)
		newPath := filepath.Join(dir, newName)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			logger.Get().Debug().
				Str("original", originalPath).
				Str("new", newPath).
				Msg("Local file already exists, using new name")
			return newPath
		}
	}

	return originalPath
}

func getUniqueRemotePath(sftpClient *sftp.Client, originalPath string) string {
	if _, err := sftpClient.Stat(originalPath); err != nil {
		return originalPath
	}

	dir := path.Dir(originalPath)
	ext := path.Ext(originalPath)
	name := strings.TrimSuffix(path.Base(originalPath), ext)

	for i := 1; i <= 1000; i++ {
		newName := fmt.Sprintf("%s (%d)%s", name, i, ext)
		newPath := path.Join(dir, newName)
		if _, err := sftpClient.Stat(newPath); err != nil {
			logger.Get().Debug().
				Str("original", originalPath).
				Str("new", newPath).
				Msg("Remote file already exists, using new name")
			return newPath
		}
	}

	return originalPath
}

func (c *SSHClient) CreateEmptyFile(remotePath string) (*sftp.File, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.sftpClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	file, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create empty file: %w", err)
	}

	return file, nil
}
