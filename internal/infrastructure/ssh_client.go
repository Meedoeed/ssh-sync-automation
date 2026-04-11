package infrastructure

import (
	"context"
	"fmt"
	"io"
	"os"
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
}

type SSHClientInterface interface {
	Connect(server *domain.Server) error
	Close() error
	IsConnected() bool
	ListFiles(remotePath string) ([]string, error)
	DownloadResumable(remotePath, localPath string) error
	DownloadWithRetry(remotePath, localPath string) error
	UploadResumable(localPath, remotePath string) error
	UploadWithRetry(localPath, remotePath string) error
	DeleteFile(remotePath string) error
}

func NewSSHClient(cfg *config.SyncCfg) *SSHClient {
	return &SSHClient{
		config: cfg,
	}
}

func (c *SSHClient) Connect(server *domain.Server) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

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
		User: server.Username,
		Auth: methods,
		// TODO: Добавить проверку known_hosts
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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
	var lastErr error
	for attempt := 0; attempt < c.config.RetryMaxAtmpt; attempt++ {
		if attempt > 0 {
			logger.Get().Warn().
				Int("attempt", attempt+1).
				Int("max_attempts", c.config.RetryMaxAtmpt).
				Str("file", remotePath).
				Msg("Retrying download")
			time.Sleep(c.config.RetryDelay)
		}

		err := c.DownloadResumable(remotePath, localPath)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("download failed after %d attempts: %w", c.config.RetryMaxAtmpt, lastErr)
}

func (c *SSHClient) UploadWithRetry(localPath, remotePath string) error {
	var lastErr error
	for attempt := 0; attempt < c.config.RetryMaxAtmpt; attempt++ {
		if attempt > 0 {
			logger.Get().Warn().
				Int("attempt", attempt+1).
				Int("max_attempts", c.config.RetryMaxAtmpt).
				Str("file", localPath).
				Msg("Retrying upload")
			time.Sleep(c.config.RetryDelay)
		}

		err := c.UploadResumable(localPath, remotePath)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("upload failed after %d attempts: %w", c.config.RetryMaxAtmpt, lastErr)
}
